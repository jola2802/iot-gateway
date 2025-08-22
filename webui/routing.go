package webui

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// showRoutingPage shows the routing page
func showRoutingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "data-forwarding.html", nil)
}
