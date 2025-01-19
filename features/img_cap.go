package features

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

func StartImageCapture(c *opcua.Client, methodGrabImageParent, methodGrabImageNode, imageNode, imageCapturedNode, imageReadCompleteNode, fileDropPath, datapointId, uploadURL string, timeout string, arg0, arg1 string) {
	//Convert timeout from string to time.Duration
	parsedTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		logrus.Errorf("Failed to parse timeout: %v", err)
	}

	ctx := context.Background()

	for {
		err := captureAndUploadImage(ctx, c, methodGrabImageParent, methodGrabImageNode, imageCapturedNode, imageNode, imageReadCompleteNode, arg0, arg1, parsedTimeout, fileDropPath)
		if err != nil {
			logrus.Printf("Error: %v", err)
		}
		time.Sleep(10 * time.Second) // Wartezeit zwischen den Bildaufnahmen

		// Nach der Bildaufnahme, versuche ein Bild aus dem Ordner hochzuladen
		uploadImageFromFolder(fileDropPath, datapointId, uploadURL)
	}
}

func captureAndUploadImage(ctx context.Context, c *opcua.Client, methodGrabImageParent, methodGrabImageNode, imageCapturedNode, imageNode, imageReadCompleteNode, arg0, arg1 string, timeout time.Duration, fileDropPath string) error {
	// Bildaufnahme initiieren
	methodParentNodeID, err := ua.ParseNodeID(methodGrabImageParent)
	if err != nil {
		return fmt.Errorf("failed to parse methodGrabImageParent node ID: %v", err)
	}
	methodNodeNodeID, err := ua.ParseNodeID(methodGrabImageNode)
	if err != nil {
		return fmt.Errorf("failed to parse methodGrabImageNode node ID: %v", err)
	}

	in0 := ua.MustVariant(arg0)
	in1 := ua.MustVariant(arg1)
	req := &ua.CallMethodRequest{
		ObjectID:       methodParentNodeID,
		MethodID:       methodNodeNodeID,
		InputArguments: []*ua.Variant{in0, in1},
	}

	resp, err := c.Call(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call GrabImage method: %v", err)
	}
	if got, want := resp.StatusCode, ua.StatusOK; got != want {
		return fmt.Errorf("got status %v, want %v", got, want)
	}
	logrus.Println("GrabImage finished")

	// Warten, bis das Bild erfasst wurde
	if err := awaitImageCapture(ctx, c, imageCapturedNode, timeout); err != nil {
		return err
	}

	// Bild herunterladen
	imageData, err := downloadImage(ctx, c, imageNode)
	if err != nil {
		return err
	}

	// Bild auf der Festplatte speichern
	err = saveImageToFile(imageData, fileDropPath)
	if err != nil {
		return err
	}

	// Setze den Knoten, dass das Bild gelesen wurde
	if err := setImageReadComplete(ctx, c, imageReadCompleteNode); err != nil {
		return err
	}

	return nil
}

func awaitImageCapture(ctx context.Context, c *opcua.Client, imageCapturedNode string, timeout time.Duration) error {
	nodeID, err := ua.ParseNodeID(imageCapturedNode)
	if err != nil {
		return fmt.Errorf("failed to parse imageCapturedNode node ID: %v", err)
	}
	node := c.Node(nodeID)
	logrus.Println("Waiting for image capture...")

	timeoutChan := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("image capture timed out")
		case <-ticker.C:
			value, err := node.Value(ctx)
			if err != nil {
				return fmt.Errorf("failed to read capture status: %v", err)
			}
			captured, ok := value.Value().(bool)
			if ok && captured {
				logrus.Println("Image capture detected")
				return nil
			}
		}
	}
}

func downloadImage(ctx context.Context, c *opcua.Client, imageNode string) ([]byte, error) {
	nodeID, err := ua.ParseNodeID(imageNode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imageNode node ID: %v", err)
	}
	node := c.Node(nodeID)
	value, err := node.Value(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %v", err)
	}
	imageData, ok := value.Value().([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid image data")
	}
	logrus.Println("Image downloaded")
	return imageData, nil
}

func saveImageToFile(imageData []byte, fileDropPath string) error {
	fileName := fmt.Sprintf("%s%d_image.png", fileDropPath, time.Now().Unix())
	err := os.WriteFile(fileName, imageData, 0644)
	if err != nil {
		return fmt.Errorf("failed to save image to file: %v", err)
	}
	logrus.Printf("Image saved to file: %s", fileName)
	return nil
}

func setImageReadComplete(ctx context.Context, c *opcua.Client, imageReadCompleteNode string) error {
	nodeID, err := ua.ParseNodeID(imageReadCompleteNode)
	if err != nil {
		return fmt.Errorf("failed to parse imageReadCompleteNode node ID: %v", err)
	}
	value := true
	v, err := ua.NewVariant(value)

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      nodeID,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue,
					Value:        v,
				},
			},
		},
	}

	resp, err := c.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to write value to imageReadCompleteNode: %v", err)
	}

	// Überprüfe den Statuscode der Antwort
	if got, want := resp.Results[0], ua.StatusOK; got != want {
		return fmt.Errorf("failed to set imageReadCompleteNode: got status %v, want %v", got, want)
	}

	logrus.Println("Image read completed, node set to true")
	return nil
}

func uploadImageFromFolder(fileDropPath, datapointId, uploadURL string) {
	files, err := os.ReadDir(fileDropPath)
	if err != nil {
		logrus.Printf("Could not read folder: %v", err)
		return
	}

	if len(files) == 0 {
		logrus.Println("No images found in the folder")
		return
	}

	// Nimm die erste Datei im Ordner
	firstImage := filepath.Join(fileDropPath, files[0].Name())
	logrus.Printf("Uploading image: %s", firstImage)

	// Datei öffnen und lesen
	img, err := os.ReadFile(firstImage)
	if err != nil {
		logrus.Printf("Could not read image file: %v", err)
		return
	}

	// Erstelle HTTP-POST Anfrage
	headers := map[string]string{
		"READ_BLOBLTIMESTAMP":    time.Now().UTC().Format("2006-01-02 15:04:05"),
		"READ_BLOB_DATAPOINT_ID": datapointId,
	}

	// Sende die Bilddaten
	err = uploadImage(img, headers, uploadURL)
	if err != nil {
		logrus.Printf("Failed to upload image: %v", err)
	}
}

func uploadImage(img []byte, headers map[string]string, uploadURL string) error {
	logrus.Println("Uploading image...")

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(img))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %v", err)
	}
	// Füge die Header hinzu
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload image: %v", err)
	}
	defer resp.Body.Close()

	logrus.Printf("Image uploaded with response: %v", resp.Status)
	return nil
}
