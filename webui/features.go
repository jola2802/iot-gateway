package webui

import (
	"context"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"github.com/sirupsen/logrus"
)

var NodeRED_URL string

type NodeDef struct {
	NodeID      *ua.NodeID
	NodeClass   ua.NodeClass
	BrowseName  string
	Description string
	Path        string
}

// join fügt den aktuellen Pfad mit dem neuen Knoten zusammen
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// browse durchläuft die Knoten eines OPC UA Servers rekursiv
func browse(ctx context.Context, n *opcua.Node, path string, level int) ([]NodeDef, error) {
	if level > 10 {
		return nil, nil
	}

	attrs, err := n.Attributes(ctx, ua.AttributeIDNodeClass, ua.AttributeIDBrowseName, ua.AttributeIDDescription)
	if err != nil {
		return nil, err
	}

	var def = NodeDef{
		NodeID: n.ID,
	}

	if attrs[0].Status == ua.StatusOK {
		def.NodeClass = ua.NodeClass(attrs[0].Value.Int())
	}

	if attrs[1].Status == ua.StatusOK {
		def.BrowseName = attrs[1].Value.String()
	}

	if attrs[2].Status == ua.StatusOK {
		def.Description = attrs[2].Value.String()
	}

	def.Path = join(path, def.BrowseName)

	var nodes []NodeDef
	if def.NodeClass == ua.NodeClassVariable {
		nodes = append(nodes, def)
	}

	browseChildren := func(refType uint32) error {
		refs, err := n.ReferencedNodes(ctx, refType, ua.BrowseDirectionForward, ua.NodeClassAll, true)
		if err != nil {
			return err
		}
		for _, rn := range refs {
			children, err := browse(ctx, rn, def.Path, level+1)
			if err != nil {
				return err
			}
			nodes = append(nodes, children...)
		}
		return nil
	}

	if err := browseChildren(id.HasComponent); err != nil {
		return nil, err
	}
	if err := browseChildren(id.Organizes); err != nil {
		return nil, err
	}
	if err := browseChildren(id.HasProperty); err != nil {
		return nil, err
	}
	return nodes, nil
}

// browseNodes ist der Endpunkt, der die Knoten eines Geräts durchsucht und als JSON zurückgibt
func browseNodes(c *gin.Context) {
	// Hol den Gerätenamen aus der URL
	deviceID := c.Param("deviceID")

	// Datenbankverbindung holen
	db, err := getDBConnection(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to database", "error": err.Error()})
		return
	}

	// Geräteeinstellungen abrufen (Adresse, Sicherheitsrichtlinie und Modus)
	query := "SELECT address, security_policy, security_mode FROM devices WHERE id = ?"
	row := db.QueryRow(query, deviceID)

	var address, securityPolicy, securityMode string
	if err := row.Scan(&address, &securityPolicy, &securityMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error fetching device settings", "error": err.Error()})
		return
	}

	// OPC UA Client konfigurieren
	client, err := opcua.NewClient(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating OPC UA client", "error": err.Error()})
		return
	}
	ctx := context.Background()

	// Verbinde mit dem OPC UA Server
	if err := client.Connect(ctx); err != nil {
		logrus.Errorf("Failed to connect to OPC UA server: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error connecting to OPC UA server", "error": err.Error()})
		return
	}
	defer client.Close(ctx)

	// Root-Knoten durchsuchen
	id, err := ua.ParseNodeID("ns=0;i=84") // Standard-Root-Knoten
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid node ID", "error": err.Error()})
		return
	}

	// Durchsuche die Knoten rekursiv
	nodeList, err := browse(ctx, client.Node(id), "", 0)
	if err != nil {
		logrus.Errorf("Error browsing OPC UA server: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Error browsing nodes", "error": err.Error()})
		return
	}

	// Enferne alle Nodes die nicht vom Typ Variable sind
	nodeList = removeNonVariableNodes(nodeList)

	// Knoten als JSON zurückgeben
	c.JSON(http.StatusOK, gin.H{"nodes": nodeList})
}

func removeNonVariableNodes(nodeList []NodeDef) []NodeDef {
	for _, node := range nodeList {
		if node.NodeClass != ua.NodeClassVariable {
			nodeList = append(nodeList, node)
		}
	}
	return nodeList
}

func getNodeRedURL(c *gin.Context) {
	// check if NodeRED_URL is set
	if NodeRED_URL != "" {
		c.JSON(http.StatusOK, gin.H{"nodeRedURL": NodeRED_URL})
		logrus.Infof("NodeRED_URL: %s", NodeRED_URL)
		return
	} else {
		NodeRED_URL = os.Getenv("NODE_RED_URL")
		logrus.Infof("NodeRED_URL set from env: %s", NodeRED_URL)
		c.JSON(http.StatusOK, gin.H{"nodeRedURL": NodeRED_URL})
	}
}
