package grpcclient

import (
	"context"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/Rudd-O/curvetls"
	"google.golang.org/grpc"

	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
)

// CoChairClient is a wrapper around our generated gRPC server.ProxyClient interface.
type CoChairClient struct {
	conf ClientConfig
	pc   server.ProxyClient
}

// NewCoChairClient returns a CoChairClient
func NewCoChairClient(conf ClientConfig) (*CoChairClient, error) {
	spub, err := curvetls.PubkeyFromString(conf.ServerPubKey)
	if err != nil {
		return nil, err
	}

	pub, err := curvetls.PubkeyFromString(conf.PubKey)
	if err != nil {
		return nil, err
	}
	priv, err := curvetls.PrivkeyFromString(conf.PrivKey)
	if err != nil {
		return nil, err
	}
	creds := curvetls.NewGRPCClientCredentials(spub, pub, priv)
	conn, err := grpc.Dial(conf.ServerIP, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}

	pc := server.NewProxyClient(conn)

	return &CoChairClient{conf, pc}, nil
}

// Put ...
func (c *CoChairClient) Put(domain string, ips []string) error {
	req := server.Backend{
		Domain: domain,
		Ips:    ips,
		// healthcheck todo
	}

	result, err := c.pc.Put(context.TODO(), &req)
	if err != nil {
		return err
	}
	fmt.Println("status:", result.Status)
	return nil
}

// State reports on the state of the proxy.
func (c *CoChairClient) State(domain string) error {
	req := server.StateRequest{
		Domain: domain,
	}

	proxyState, err := c.pc.State(context.TODO(), &req)
	if err != nil {
		return err
	}
	fmt.Println("Configured upstreams:")
	fmt.Println("---")
	printUpstream := func(be *server.Backend) {
		fmt.Println("domain:", be.Domain)
		for _, ip := range be.Ips {
			fmt.Println("\t", ip)
		}
		fmt.Println("---")
	}
	for _, be := range proxyState.Backends {
		printUpstream(be)
	}
	return nil
}

// ClientConfig maps our config for a pure grpc client.
type ClientConfig struct {
	PubKey       string `toml:"client_public_key"`
	PrivKey      string `toml:"client_private_key"`
	ServerPubKey string `toml:"server_public_key"`
	ServerIP     string `toml:"server_ip"`
}

// NewClientConfig ...
func NewClientConfig(path string) (ClientConfig, error) {
	var conf ClientConfig
	if path == "" {
		// default client config path
		path = os.ExpandEnv("$HOME/.config/co-chair/client.toml")
	}
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return conf, err
	}
	return conf, nil
}
