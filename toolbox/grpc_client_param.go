package toolbox

import (
	"google.golang.org/grpc/credentials"
	"errors"
	"google.golang.org/grpc"
)

// GRPCClientParam is a parameter struct for gRPC clients.
type GRPCClientParam struct {
	ServerEndpoint     string `param:"desc=Server endpoint;default=localhost:10000"` // Host:port address of the server
	TLS                bool   `param:"desc=Enable TLS;default=false"`                // TLS enabled
	CAFile             string `param:"desc=CA certificate file;file"`                // CA cert file
	ServerHostOverride string `param:"desc=Host name override for certificate"`      // Server name returned from the TLS handshake (for debugging)
}

// GetGRPCDialOpts returns dial options for a gRPC client based on the client parameters
func GetGRPCDialOpts(config GRPCClientParam) ([]grpc.DialOption, error) {
	if !config.TLS {
		return []grpc.DialOption{grpc.WithInsecure()}, nil
	}

	if config.CAFile == "" {
		return nil, errors.New("missing CA file for TLS")
	}

	creds, err := credentials.NewClientTLSFromFile(config.CAFile, config.ServerHostOverride)
	if err != nil {
		return nil, err
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, nil
}