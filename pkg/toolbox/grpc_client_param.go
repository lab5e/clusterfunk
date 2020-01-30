package toolbox

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCClientParam is a parameter struct for gRPC clients.
type GRPCClientParam struct {
	ServerEndpoint     string `kong:"help='Server endpoint',default='localhost:10000'"` // Host:port address of the server
	TLS                bool   `kong:"help='Enable TLS',default='false'"`                // TLS enabled
	CAFile             string `kong:"help='CA certificate file',type='existingfile'"`   // CA cert file
	ServerHostOverride string `kong:"help='Host name override for certificate'"`        // Server name returned from the TLS handshake (for debugging)
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
