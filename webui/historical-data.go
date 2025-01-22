package webui

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func showHistoricalDataPage(c *gin.Context) {
	c.HTML(http.StatusOK, "historical-data.html", nil)
}
