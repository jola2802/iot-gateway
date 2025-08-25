package opcua

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awcullen/opcua/client"
	awcullenua "github.com/awcullen/opcua/ua"
	"github.com/sirupsen/logrus"
)

func readData(ch *client.Client, nodes []DataNode) ([]*awcullenua.DataValue, error) {
	if ch == nil {
		return nil, errors.New("OPC-UA: client not connected")
	}

	readRequest := &awcullenua.ReadRequest{
		NodesToRead:        make([]awcullenua.ReadValueID, len(nodes)),
		TimestampsToReturn: awcullenua.TimestampsToReturnBoth,
	}

	for i, dn := range nodes {
		parsedNodeID := awcullenua.ParseNodeID(dn.Node)
		readRequest.NodesToRead[i] = awcullenua.ReadValueID{
			NodeID:      parsedNodeID,
			AttributeID: awcullenua.AttributeIDValue,
		}
	}

	ctx := context.Background()
	readResponse, err := ch.Read(ctx, readRequest)
	if err != nil {
		// logrus.Errorf("OPC-UA: reading data failed: %v", err)
		return nil, errors.New("reading data failed")
	}
	// var errorMessages []string

	var successfulResults []*awcullenua.DataValue

	for i, result := range readResponse.Results {
		if !result.StatusCode.IsGood() {
			logrus.Errorf("OPC-UA: reading node '%s' failed with status: %v", nodes[i], result.StatusCode)
			continue
		}
		successfulResults = append(successfulResults, &result)
	}

	return successfulResults, nil
}

// readDataWithRetry liest Daten mit Retry-Mechanismus für bessere Verbindungsqualität
func readDataWithRetry(ch *client.Client, nodes []DataNode, maxRetries int) ([]*awcullenua.DataValue, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		data, err := readData(ch, nodes)
		if err == nil {
			return data, nil
		}

		lastErr = err
		if attempt < maxRetries-1 {
			logrus.Warnf("OPC-UA: Read attempt %d/%d failed: %v. Retrying...", attempt+1, maxRetries, err)
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond) // Exponential backoff
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %v", maxRetries, lastErr)
}

// ---------------------- UNUSED --------------------------
// UpdateDataNode aktualisiert einen OPC-UA-Datenpunkt
func UpdateDataNode(ch *client.Client, nodeID string, value interface{}) error {
	if ch == nil {
		return errors.New("client not connected")
	}

	parsedNodeID := awcullenua.ParseNodeID(nodeID)

	writeRequest := &awcullenua.WriteRequest{
		NodesToWrite: []awcullenua.WriteValue{
			{
				NodeID:      parsedNodeID,
				AttributeID: awcullenua.AttributeIDValue,
				Value: awcullenua.DataValue{
					Value: value,
				},
			},
		},
	}

	ctx := context.Background()
	writeResponse, err := ch.Write(ctx, writeRequest)
	if err != nil {
		logrus.Errorf("write failed: %v", err)
		return errors.New("write failed")
	}

	if !writeResponse.Results[0].IsGood() {
		logrus.Errorf("write failed with status: %v", writeResponse.Results[0])
		return errors.New("write failed with status")
	}

	logrus.Errorf("OPC-UA: data node '%s' updated successfully", nodeID)
	return nil
}

// GetNodeName ruft den Namen einer Node basierend auf der NodeID ab
func GetNodeName(ch *client.Client, nodeID string) (string, error) {
	parsedNodeID := awcullenua.ParseNodeID(nodeID)

	req := &awcullenua.ReadRequest{
		NodesToRead: []awcullenua.ReadValueID{
			{
				NodeID:      parsedNodeID,
				AttributeID: awcullenua.AttributeIDDisplayName,
			},
		},
	}

	ctx := context.Background()
	resp, err := ch.Read(ctx, req)
	if err != nil {
		logrus.Errorf("OPC-UA: failed to read node name: %v", err)
		return "", errors.New("failed to read node name")
	}

	if !resp.Results[0].StatusCode.IsGood() {
		logrus.Errorf("OPC-UA: failed to read node name with status: %v", resp.Results[0].StatusCode)
		return "", errors.New("failed to read node name with status")
	}

	switch v := resp.Results[0].Value.(type) {
	case awcullenua.LocalizedText:
		return v.Text, nil
	case *awcullenua.LocalizedText:
		return v.Text, nil
	case string:
		return v, nil
	default:
		logrus.Errorf("OPC-UA: unexpected type for display name: %T", v)
		return "", errors.New("unexpected type for display name")
	}
}
