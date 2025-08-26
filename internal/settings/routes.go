package settings

import "github.com/gin-gonic/gin"

// RegisterRoutes registers settings-related routes on the given router group
func RegisterRoutes(settingsGroup *gin.RouterGroup, handler *SettingsHandler) {
	// Redirect main settings page to profile (default)
	settingsGroup.GET("", func(c *gin.Context) {
		c.Redirect(302, "/settings/profile")
	})

	// Profile settings
	settingsGroup.GET("/profile", handler.GetProfileSettingsPage)
	settingsGroup.POST("/profile/personal", handler.HandleCreateProfile)
	settingsGroup.POST("/profile/online", handler.HandleUpdateOnlineProfile)
	settingsGroup.POST("/profile/context", handler.HandleUpdateContext)
	settingsGroup.POST("/profile/parse-cv", handler.HandleCVUpload)

	// Account settings
	settingsGroup.GET("/account", handler.GetAccountSettingsPage)
	settingsGroup.POST("/account/update", handler.HandleUpdateAccount)
	settingsGroup.POST("/account/extension-password", handler.HandleUpdateExtensionPassword)
	settingsGroup.DELETE("/account/delete", handler.DeleteAccount)

	// Experience routes
	settingsGroup.GET("/profile/experience/new", handler.GetAddExperiencePage)
	settingsGroup.GET("/profile/experience/:id/edit", handler.GetEditExperiencePage)
	settingsGroup.POST("/profile/experience", handler.HandleExperienceForm)
	settingsGroup.POST("/profile/experience/:id", handler.HandleUpdateExperienceForm)
	// Support both methods for maximum compatibility
	settingsGroup.DELETE("/profile/experience/:id", handler.HandleDeleteWorkExperience)
	settingsGroup.POST("/profile/experience/:id/delete", handler.HandleDeleteWorkExperience)

	// Education routes
	settingsGroup.GET("/profile/education/new", handler.GetAddEducationPage)
	settingsGroup.GET("/profile/education/:id/edit", handler.GetEditEducationPage)
	settingsGroup.POST("/profile/education", handler.CreateEducationForm)
	settingsGroup.POST("/profile/education/:id", handler.HandleUpdateEducationForm)
	// Support both methods for maximum compatibility
	settingsGroup.DELETE("/profile/education/:id", handler.HandleDeleteEducation)
	settingsGroup.POST("/profile/education/:id/delete", handler.HandleDeleteEducation)

	// Certification routes
	settingsGroup.GET("/profile/certification/new", handler.GetAddCertificationPage)
	settingsGroup.GET("/profile/certification/:id/edit", handler.GetEditCertificationPage)
	settingsGroup.POST("/profile/certification", handler.CreateCertificationForm)
	settingsGroup.POST("/profile/certification/:id", handler.HandleUpdateCertificationForm)
	// Support both methods for maximum compatibility
	settingsGroup.DELETE("/profile/certification/:id", handler.HandleDeleteCertification)
	settingsGroup.POST("/profile/certification/:id/delete", handler.HandleDeleteCertification)

	// Quota routes
	settingsGroup.GET("/quotas", handler.GetQuotasPage)
}
