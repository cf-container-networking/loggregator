package clientreader

import "fmt"

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	SetAddresses(addresses []string) int
}

func Read(clientPool map[string]ClientPool, protocols []string, dopplers map[string][]string) string {
	protocol, servers := chooseProtocol(protocols, dopplers)
	if protocol == "" {
		panic(fmt.Sprintf("No dopplers listening on %v", protocols))
	}
	clients := clientPool[protocol].SetAddresses(servers)
	if clients == 0 {
		panic(fmt.Sprintf("Unable to connect to dopplers running on %s", protocol))
	}
	return protocol
}

func chooseProtocol(protocols []string, dopplers map[string][]string) (string, []string) {
	for _, protocol := range protocols {
		var result []string
		switch protocol {
		case "udp":
			result = dopplers["udp"]
		case "tcp":
			result = dopplers["tcp"]
		case "tls":
			result = dopplers["tls"]
		}
		if len(result) > 0 {
			return protocol, result
		}
	}
	return "", nil
}
