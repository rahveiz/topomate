package project

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/utils"
)

type Net struct {
	IPNet            *net.IPNet
	NextAvailable    *net.IPNet
	AvailableSubnets int
	AutoAddress      bool
}

func (n Net) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.IPNet.String())
}

func (n *Net) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	_, ipnet, err := net.ParseCIDR(s)
	if err == nil {
		n.IPNet = ipnet
		return nil
	}
	return err
}

func NewNetwork(prefix string, prefixLen int) Net {
	_, n, err := net.ParseCIDR(prefix)
	if err != nil {
		utils.Fatalln("NewNetwork:", err)
	}
	var subLen int
	cur, max := n.Mask.Size()
	if prefixLen > 0 {
		subLen = prefixLen - cur
	} else {
		subLen = max - 2 - cur
		prefixLen = max - 2
	}
	s, err := cidr.Subnet(n, subLen, 0)
	if err != nil {
		utils.Fatalln("NewNetwork:", err)
	}
	return Net{
		IPNet:            n,
		NextAvailable:    s,
		AvailableSubnets: int(math.Pow(2, float64(prefixLen-cur))),
		AutoAddress:      true,
	}
}

// AllIPs returns a slice containing all IPs in a network
// (identifier and broadcast included)
func (n Net) AllIPs() []net.IP {
	nbHosts := n.Size()
	hosts := make([]net.IP, nbHosts)
	mask := binary.BigEndian.Uint32(n.IPNet.Mask)
	start := binary.BigEndian.Uint32(n.IPNet.IP)

	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)

	// loop through addresses as uint32
	cnt := 0
	for i := start; i <= finish; i++ {
		// fmt.Println("pok")
		// convert back to net.IP
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		hosts[cnt] = ip
		cnt++
	}
	return hosts
}

// Size returns the size of a network (number of addresses)
func (n Net) Size() int {
	m, _ := n.IPNet.Mask.Size()
	return int(math.Pow(2, 32-float64(m)))
}

// Hosts returns a slice of hosts in a network
func (n Net) Hosts() []net.IP {
	return n.AllIPs()[1 : n.Size()-1]
}

// NextSubnet returns the current NextAvailable IPNet, then sets the value to
// the next subnet
func (n *Net) NextSubnet(prefixLen int) net.IPNet {
	res := *n.NextAvailable
	if n.AvailableSubnets < 1 {
		utils.Fatalf("Network %s: no more subnets of size %d available\n", n.IPNet.String(), prefixLen)
	}
	_n, full := cidr.NextSubnet(n.NextAvailable, prefixLen)
	if full {
		utils.Fatalln("NextIP: Subnet full", n.NextAvailable.String())
	}
	n.NextAvailable = _n
	n.AvailableSubnets--
	return res
}

// NextLinkIPs returns the 2 first host IPs of the NextAvailable IPNet, then
// sets the value to the next one
func (n *Net) NextLinkIPs() (a net.IPNet, b net.IPNet) {
	subLen, _ := n.NextAvailable.Mask.Size()
	if n.Is4() {
		_n := n.NextSubnet(subLen)
		_n.IP = cidr.Inc(_n.IP)
		a = _n
		_n.IP = cidr.Inc(_n.IP)
		b = _n
	} else {
		_n := n.NextSubnet(subLen)
		_n.IP = cidr.Inc(_n.IP)
		a = _n
		_n.IP = cidr.Inc(_n.IP)
		b = _n
	}
	return
}

// NextIP returns the current NextAvailable IPNet, then increments its IP by one
func (n *Net) NextIP() net.IPNet {
	res := *n.NextAvailable
	n.NextAvailable.IP = cidr.Inc(n.NextAvailable.IP)
	return res
}

// Is4 returns true if Net is an IPV4 network
func (n Net) Is4() bool {
	return n.IPNet.IP.To4() != nil
}

// CheckPrefix returns the subnet length of the network. The second value
// return is true if the prefixLen provided is valable.
func (n Net) CheckPrefix(prefixLen int) (int, bool) {
	m, max := n.IPNet.Mask.Size()
	return m, !(prefixLen < m || prefixLen > max)
}
