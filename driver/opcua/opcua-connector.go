package opcua

import (
	"context"
	"errors"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/sirupsen/logrus"
)

func readData(ch *client.Client, nodes []DataNode) ([]*ua.DataValue, error) {
	if ch == nil {
		return nil, errors.New("OPC-UA: client not connected")
	}

	readRequest := &ua.ReadRequest{
		NodesToRead:        make([]ua.ReadValueID, len(nodes)),
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	for i, dn := range nodes {
		parsedNodeID := ua.ParseNodeID(dn.Node)
		readRequest.NodesToRead[i] = ua.ReadValueID{
			NodeID:      parsedNodeID,
			AttributeID: ua.AttributeIDValue,
		}
	}

	ctx := context.Background()
	readResponse, err := ch.Read(ctx, readRequest)
	if err != nil {
		// logrus.Errorf("OPC-UA: reading data failed: %v", err)
		return nil, errors.New("reading data failed")
	}
	// var errorMessages []string

	var successfulResults []*ua.DataValue

	for i, result := range readResponse.Results {
		if !result.StatusCode.IsGood() {
			logrus.Errorf("OPC-UA: reading node '%s' failed with status: %v", nodes[i], result.StatusCode)
			continue
		}
		successfulResults = append(successfulResults, &result)
	}

	return successfulResults, nil
}

// ---------------------- UNUSED --------------------------
// UpdateDataNode aktualisiert einen OPC-UA-Datenpunkt
func UpdateDataNode(ch *client.Client, nodeID string, value interface{}) error {
	if ch == nil {
		return errors.New("client not connected")
	}

	parsedNodeID := ua.ParseNodeID(nodeID)

	writeRequest := &ua.WriteRequest{
		NodesToWrite: []ua.WriteValue{
			{
				NodeID:      parsedNodeID,
				AttributeID: ua.AttributeIDValue,
				Value: ua.DataValue{
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
	parsedNodeID := ua.ParseNodeID(nodeID)

	req := &ua.ReadRequest{
		NodesToRead: []ua.ReadValueID{
			{
				NodeID:      parsedNodeID,
				AttributeID: ua.AttributeIDDisplayName,
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
