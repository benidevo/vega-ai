package documents

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *DocumentHandler, authMiddleware gin.HandlerFunc, csrfMiddleware gin.HandlerFunc) {
	documentRoutes := router.Group("/documents")
	documentRoutes.Use(authMiddleware)
	{
		documentRoutes.GET("", handler.GetDocumentsHub)
		documentRoutes.GET("/partial", handler.GetDocumentPartial)
		documentRoutes.GET("/:id/export", handler.ExportDocument)
		documentRoutes.POST("/save", csrfMiddleware, handler.SaveDocument)
		documentRoutes.DELETE("/:id", csrfMiddleware, handler.DeleteDocument)
	}
}
