package opcua

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

// InitClient initialisiert den OPC-UA-Client
func InitClient(address, securityPolicy string, securityMode ua.MessageSecurityMode, certFile string, keyFile string, username, password string) (*opcua.Client, error) {
	// Optionen initialisieren
	opts := []opcua.Option{
		opcua.SecurityMode(securityMode),
		opcua.SecurityPolicy(securityPolicy),
		opcua.AutoReconnect(true),
		opcua.ReconnectInterval(15 * time.Second),
		opcua.RequestTimeout(10 * time.Second),
	}

	// Zertifikat und Private Key für Sicherheit einbinden, wenn nötig
	if securityMode != ua.MessageSecurityModeNone && securityPolicy != "None" {
		if certFile != "" && keyFile != "" {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				logrus.Errorf("OPC-UA: failed to load client certificate and key: %v", err)
				return nil, errors.New("failed to load client certificate and key")
			}

			// Typumwandlung des privaten Schlüssels
			privateKey, ok := cert.PrivateKey.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.New("OPC-UA: invalid private key type; expected RSA private key")
			}

			// Füge das Zertifikat und den privaten Schlüssel zu den Optionen hinzu
			opts = append(opts, opcua.Certificate(cert.Certificate[0]), opcua.PrivateKey(privateKey))
		} else {
			logrus.Warn("Security mode and policy require a certificate and private key, but none were provided")
		}
	}

	// OPC-UA Client erstellen
	client, err := opcua.NewClient(address, opts...)
	if err != nil {
		logrus.Errorf("OPC-UA: failed to create OPC-UA client: %v", err)
		return nil, errors.New("failed to create OPC-UA client")
	}
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("OPC-UA: failed to connect to OPC-UA server: %v", err)
		return nil, errors.New("failed to connect to OPC-UA server")
	}

	return client, nil
}

func ReadData(client *opcua.Client, selectedNodes []string) ([]*ua.DataValue, error) {
	if client == nil {
		return nil, errors.New("OPC-UA: client not connected")
	}

	readRequest := &ua.ReadRequest{
		NodesToRead:        make([]*ua.ReadValueID, len(selectedNodes)),
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	for i, nodeID := range selectedNodes {
		parsedNodeID, err := ua.ParseNodeID(nodeID)
		if err != nil {
			logrus.Errorf("OPC-UA: failed to parse node ID '%s': %v", nodeID, err)
			return nil, errors.New("failed to parse node ID")
		}
		readRequest.NodesToRead[i] = &ua.ReadValueID{
			NodeID:      parsedNodeID,
			AttributeID: ua.AttributeIDValue,
		}
	}

	ctx := context.Background()
	readResponse, err := client.Read(ctx, readRequest)
	if err != nil {
		logrus.Errorf("OPC-UA: reading data failed: %v", err)
		return nil, errors.New("reading data failed")
	}
	// var errorMessages []string

	var successfulResults []*ua.DataValue

	for i, result := range readResponse.Results {
		if result.Status != ua.StatusOK {
			logrus.Errorf("OPC-UA: reading node '%s' failed with status: %v", selectedNodes[i], result.Status)
			continue
		}
		successfulResults = append(successfulResults, result)
	}

	return successfulResults, nil
}

// UpdateDataNode aktualisiert einen OPC-UA-Datenpunkt
func UpdateDataNode(client *opcua.Client, nodeID string, value interface{}) error {
	if client == nil {
		return errors.New("client not connected")
	}

	parsedNodeID, err := ua.ParseNodeID(nodeID)
	if err != nil {
		logrus.Errorf("failed to parse node ID '%s': %v", nodeID, err)
		return errors.New("failed to parse node ID")
	}

	writeRequest := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      parsedNodeID,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					Value: ua.MustVariant(value),
				},
			},
		},
	}

	ctx := context.Background()
	writeResponse, err := client.Write(ctx, writeRequest)
	if err != nil {
		logrus.Errorf("write failed: %v", err)
		return errors.New("write failed")
	}

	if writeResponse.Results[0] != ua.StatusOK {
		logrus.Errorf("write failed with status: %v", writeResponse.Results[0])
		return errors.New("write failed with status")
	}

	logrus.Errorf("OPC-UA: data node '%s' updated successfully", nodeID)
	return nil
}

// GetNodeName ruft den Namen einer Node basierend auf der NodeID ab
func GetNodeName(client *opcua.Client, nodeID string) (string, error) {
	parsedNodeID, err := ua.ParseNodeID(nodeID)
	if err != nil {
		logrus.Errorf("OPC-UA: failed to parse node ID: %v", err)
		return "", errors.New("failed to parse node ID")
	}
	req := &ua.ReadRequest{
		NodesToRead: []*ua.ReadValueID{
			{
				NodeID:      parsedNodeID,
				AttributeID: ua.AttributeIDDisplayName,
			},
		},
	}

	ctx := context.Background()
	resp, err := client.Read(ctx, req)
	if err != nil {
		logrus.Errorf("OPC-UA: failed to read node name: %v", err)
		return "", errors.New("failed to read node name")
	}

	if resp.Results[0].Status != ua.StatusOK {
		logrus.Errorf("OPC-UA: failed to read node name with status: %v", resp.Results[0].Status)
		return "", errors.New("failed to read node name with status")
	}

	switch v := resp.Results[0].Value.Value().(type) {
	case ua.LocalizedText:
		return v.Text, nil
	case *ua.LocalizedText:
		return v.Text, nil
	case string:
		return v, nil
	default:
		logrus.Errorf("OPC-UA: unexpected type for display name: %T", v)
		return "", errors.New("unexpected type for display name")
	}
}
