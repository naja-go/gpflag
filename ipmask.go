package gpflag

import (
	"fmt"
	"net"
	"strconv"
)

// ParseIPv4Mask written in IP form (e.g. 255.255.255.0).
// This function should really belong to the net package.
func ParseIPv4Mask(s string) net.IPMask {
	mask := net.ParseIP(s)
	if mask == nil {
		if len(s) != 8 {
			return nil
		}
		// net.IPMask.String() actually outputs things like ffffff00
		// so write a horrible parser for that as well  :-(
		m := []int{}
		for i := 0; i < 4; i++ {
			b := "0x" + s[2*i:2*i+2]
			d, err := strconv.ParseInt(b, 0, 0)
			if err != nil {
				return nil
			}
			m = append(m, int(d))
		}
		s := fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
		mask = net.ParseIP(s)
		if mask == nil {
			return nil
		}
	}
	mask = mask.To4()
	if mask == nil {
		return nil
	}
	return net.IPv4Mask(mask[0], mask[1], mask[2], mask[3])
}
