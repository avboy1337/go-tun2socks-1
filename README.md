﻿# tun2socks


A tun2socks implementation written in Go.



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
