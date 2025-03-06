package webui

import (
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

// User defines a user with their associated ACL entries.
type User struct {
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	AclEntries []ACLEntry `json:"aclEntries"`
}

// ACLEntry defines the ACL topic and its associated permission for a user.
type ACLEntry struct {
	Topic      string `json:"topic"`
	Permission int    `json:"permission"` // Als int definiert
}

// Settings repräsentiert die Einstellungen der Anwendung
type Settings struct {
	DockerIP          string `json:"docker_ip"`
	UseCustomServices bool   `json:"use_custom_services"`
	NodeRedURL        string `json:"nodered_url,omitempty"`
	InfluxDBURL       string `json:"influxdb_url,omitempty"`
	UseExternalBroker bool   `json:"use_external_broker"`
	BrokerURL         string `json:"broker_url,omitempty"`
	BrokerPort        string `json:"broker_port,omitempty"`
	BrokerUsername    string `json:"broker_username,omitempty"`
	BrokerPassword    string `json:"broker_password,omitempty"`
}

// showSettingsPage shows the settings page, returning broker settings and user data.
func showSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{})
}
