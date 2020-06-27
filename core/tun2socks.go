package core

import (
	"errors"
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/adapters/gonet"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"github.com/google/netstack/tcpip/link/channel"
	"github.com/google/netstack/tcpip/network/ipv4"
	"github.com/google/netstack/tcpip/stack"
	"github.com/google/netstack/tcpip/transport/tcp"
	"github.com/google/netstack/tcpip/transport/udp"
	"github.com/google/netstack/waiter"
	"github.com/yinghuocho/gotun2socks/tun"
	"go-tun2socks/socks5"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var  wrapnet uint32;
var  mask uint32
var  relayip net.IP
var  port  uint16;

func StartTunDevice(tunDevice string,tunAddr string,tunMask string,tunGW string,mtu int,sock5Addr string,tunDNS string) {
	if(len(tunDevice)==0){
		tunDevice="tun0";
	}
	if(len(tunAddr)==0){
		tunAddr="10.0.0.2";
	}
	if(len(tunMask)==0){
		tunMask="255.255.255.0";
	}
	if(len(tunGW)==0){
		tunGW="10.0.0.1";
	}
	if(len(tunDNS)==0){
		tunDNS="114.114.114.114";
	}
	dnsServers := strings.Split(tunDNS, ",")
	var dev io.ReadWriteCloser;
	f, err:= tun.OpenTunDevice(tunDevice, tunAddr, tunGW, tunMask, dnsServers)
	if err != nil {
		fmt.Println("Error listening:", err)
		return ;
	}
	dev=f;
	ch := make(chan os.Signal, 1)
	signal.Notify(ch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-ch
		switch s {
		default:
			os.Exit(0);
		}
	}()
	ForwardTransportFromIo(dev,mtu,sock5Addr,tunDNS);
}
func ForwardTransportFromIo(dev io.ReadWriteCloser,mtu int,lSocksAddr string,tunDNS string) error {
	var nicID tcpip.NICID =1;
	macAddr, err := net.ParseMAC("de:ad:be:ee:ee:ef")
	if err != nil {
		fmt.Printf(err.Error());
		return err
	}
	//[]string{ipv4.ProtocolName, ipv6.ProtocolName, arp.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName},
	s := stack.New( stack.Options{NetworkProtocols:   []stack.NetworkProtocol{ipv4.NewProtocol()},
		TransportProtocols: []stack.TransportProtocol{tcp.NewProtocol(), udp.NewProtocol()}})
	//转发开关,必须
	s.SetForwarding(true);
	var linkID stack.LinkEndpoint
	var channelLinkID= channel.New(256, uint32(mtu),   tcpip.LinkAddress(macAddr))

	// write tun
	go func() {
		for {
			select {
			case pkt := <-channelLinkID.C:
				tmpBuf:=append(pkt.Pkt.Header.View(),pkt.Pkt.Data.ToView()...)
				if(len(tmpBuf)>0) {
					dev.Write(tmpBuf)
				}
				break;
			}
		}
	}()
	linkID=channelLinkID;
	if err != nil {
		return err
	}
	if err := s.CreateNIC(nicID, linkID); err != nil {
		return errors.New(err.String())
	}
	//promiscuous mode 必须
	s.SetPromiscuousMode(nicID, true)
	tcpForwarder := tcp.NewForwarder(s, 0, 256, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf(err.String());
			return
		}
		r.Complete(false)
		conn:=gonet.NewConn(&wq, ep)
		defer conn.Close();
		socksConn,err1:= net.Dial("tcp",lSocksAddr)
		if err1 != nil {
			log.Println(err1)
			return
		}
		defer socksConn.Close();
		if(socks5.SocksCmd(socksConn,1,remoteAddr)==nil) {
			go io.Copy(conn, socksConn)
			io.Copy(socksConn, conn)
		}
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			fmt.Printf("r.CreateEndpoint() = %v", err)
		}
		defer ep.Close()
		conn :=gonet.NewConn(&wq, ep)
		defer conn.Close();
		//dns port
		if(strings.HasSuffix(conn.LocalAddr().String(),":53")){
			dnsReq(conn,"udp",tunDNS);
		}
	})
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	// read tun data
	var buf=make([]byte,mtu)
	for {
		n, e := dev.Read(buf[:])
		if e != nil {
			fmt.Printf("e:%v\r\n",e)
			break;
		}
		tmpView:=buffer.NewVectorisedView(n,[]buffer.View{
			buffer.NewViewFromBytes(buf[:n]),
		})
		channelLinkID.InjectInbound(header.IPv4ProtocolNumber, tcpip.PacketBuffer{
			Data: tmpView,
		})
	}
	return nil
}
/*to dns*/
func dnsReq(conn *gonet.Conn,action string,dnsAddr string) error{
	if(action=="tcp"){
		dnsConn, err := net.Dial(action, dnsAddr);
		if err != nil {
			fmt.Println(err.Error())
			return err;
		}
		defer dnsConn.Close();
		go io.Copy(conn, dnsConn)
		io.Copy(dnsConn, conn)
		fmt.Printf("dnsReq Tcp\r\n");
		return nil;
	}else {
		buf := make([]byte, 4096)
		var n = 0;
		var err error;
		n, err = conn.Read(buf)
		if err != nil {
			fmt.Printf("c.Read() = %v", err)
			return err;
		}
		dnsConn, err := net.Dial("udp",dnsAddr);
		if err != nil {
			fmt.Println(err.Error())
			return err;
		}
		defer dnsConn.Close();
		_, err = dnsConn.Write(buf[:n])
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
		n, err = dnsConn.Read(buf);
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
		_, err = conn.Write(buf[:n])
		if (err != nil) {
			fmt.Println(err.Error())
			return err;
		}
	}
	return nil;
}

