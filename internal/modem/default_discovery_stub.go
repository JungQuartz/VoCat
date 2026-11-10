//go:build !linux && !darwin

package modem

func NewSystemDiscoverer() Discoverer {
	return NewSystemDiscovererWithOptions(DiscoveryOptions{})
}

func NewSystemDiscovererWithOptions(DiscoveryOptions) Discoverer {
	return unsupportedDiscoverer{}
}
