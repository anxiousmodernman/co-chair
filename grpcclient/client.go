package grpcclient

type Client struct{}

type Config struct {
}

func NewCoChairClient(conf Config) *Client {
	var c Client

	return &c
}
