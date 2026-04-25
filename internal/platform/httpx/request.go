package httpx

import "github.com/gin-gonic/gin"

func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		_ = c.Error(err)
		WriteInvalidArgument(c, invalidRequestBodyMessage)
		return false
	}

	return true
}
