package resources

import (
	"../config"
	"../models"
	"fmt"
)

func NewSentryDSN(appname, platform string) (map[string]interface{}, error) {
	app := models.GetApplication(appname)
	if app == nil {
		return nil, fmt.Errorf("No application %s found", appname)
	}
	u := fmt.Sprintf("%s/register_dsn/%s/%s/%s", config.Config.Sentrymgr,
		app.Namespace, platform, appname)
	return Get(u)
}
