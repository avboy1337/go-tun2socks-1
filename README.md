# tun2socks


A tun2socks implementation written in Go.

最新版编译不能通过，因为依赖库，google的那个git不支持了。这里测试的go版本1.2的最后一个版本可以正常编译。

# use 

var tunDevice="tun0";

var tunAddr="10.0.0.2";

var tunMask="255.255.255.0";

var tunGw="10.0.0.1";

var mtu=1500;

var socksAddr="127.0.0.1:1080";

var tunDns="127.0.0.1:53";

tun2socks.StartTunDevice(tunDevice,tunAddr, tunMask, tunGw, mtu,socksAddr,tunDns);

# thank
  github.com/google/netstack
 
  
  github.com/miekg/dns
