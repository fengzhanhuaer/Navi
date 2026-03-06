// handlers/search_engines.go
// 搜索引擎相关 HTTP 处理器

package handlers

import (
	"net/http"
	"strconv"

	"navi/db"

	"github.com/gin-gonic/gin"
)

func GetSearchEngines(c *gin.Context) {
	engines, err := db.GetSearchEngines()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if engines == nil {
		engines = []db.SearchEngine{}
	}
	c.JSON(http.StatusOK, engines)
}

func SetDefaultEngine(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := db.SetDefaultEngine(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
