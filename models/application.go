package models

import (
	"../config"
	. "../utils"
	"errors"
	"fmt"
	"github.com/astaxie/beego/orm"
	"path"
	"strings"
	"time"
)

const (
	appPathPrefix = "/NBE/"
)

type Application struct {
	Id        int
	Name      string
	Version   string
	Pname     string
	User      *User `orm:"rel(fk)"`
	Group     string
	Created   time.Time `orm:"auto_now_add;type(datetime)"`
	ImageAddr string
}

type AppYaml struct {
	Appname  string   `json:appname`
	Runtime  string   `json:runtime`
	Port     int      `json:port`
	Cmd      []string `json:cmd`
	Test     []string `json:test`
	Build    []string `json:build`
	Services []string `json:services`
	Static   string   `json:static`
	Daemon   bool     `json:daemon`
	Schema   string   `json:schema`
}

type ConfigYaml map[string]interface{}

// Application
func (self *Application) TableUnique() [][]string {
	return [][]string{
		[]string{"Name", "Version"},
	}
}

func GetApplicationById(appId int) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Id", appId).One(&app); err != nil {
		return nil
	}
	return &app
}

func NewApplication(projectname, version, group, appyaml, configyaml string) *Application {
	if configyaml == "" {
		configyaml = "{}"
	}
	var appYamlJson AppYaml
	var configYamlJson ConfigYaml

	if err1, err2 := JSONDecode(appyaml, &appYamlJson), JSONDecode(configyaml, &configYamlJson); err1 != nil || err2 != nil {
		Logger.Info("app.yaml error: ", err1)
		Logger.Info("config.yaml error: ", err2)
		return nil
	}
	Logger.Debug("app.yaml: ", appYamlJson)
	Logger.Debug("config.yaml: ", configYamlJson)

	appName := appYamlJson.Appname

	if app := GetApplicationByNameAndVersion(appName, version); app != nil {
		Logger.Info("App already registered: ", app)
		return app
	}

	// 生成新用户
	user := NewUser(appName)
	if user == nil {
		return nil
	}

	// 用户绑定应用
	app := Application{Name: appName, Version: version, Pname: projectname, Group: group, User: user}
	if _, err := db.Insert(&app); err != nil {
		Logger.Info("Create App error: ", err)
		return nil
	}

	// 清理config里的mysql/redis配置
	for key, _ := range configYamlJson {
		if strings.HasPrefix(key, "mysql") || strings.HasPrefix(key, "redis") {
			delete(configYamlJson, key)
		}
	}
	if configYaml, err := YAMLEncode(configYamlJson); err == nil {
		etcdClient.Create((&app).GetYamlPath("config"), configYaml, 0)
	}
	if appYaml, err := YAMLEncode(appYamlJson); err == nil {
		etcdClient.Create((&app).GetYamlPath("app"), appYaml, 0)
	}
	return &app
}

func GetApplicationByNameAndVersion(name, version string) *Application {
	var app Application
	if err := db.QueryTable(new(Application)).Filter("Name", name).Filter("Version", version).RelatedSel().One(&app); err != nil {
		return nil
	}
	return &app
}

func (self *Application) CreateDNS() error {
	dns := make(map[string]string)
	dns["host"] = config.Config.Masteraddr
	cpath := path.Join("/skydns/com/hunantv/intra", self.Name)
	if _, err := etcdClient.Create(cpath, "", 0); err != nil {
		return err
	}
	if r, err := JSONEncode(dns); err == nil {
		etcdClient.Set(cpath, r, 0)
		return nil
	} else {
		return err
	}
}

func (self *Application) GetYamlPath(cpath string) string {
	return path.Join(appPathPrefix, self.Name, self.Version, fmt.Sprintf("%s.yaml", cpath))
}

func (self *Application) GetAppYaml() (*AppYaml, error) {
	var appYaml AppYaml
	cpath := self.GetYamlPath("app")
	r, err := etcdClient.Get(cpath, false, false)
	if err != nil {
		return &appYaml, err
	}
	if r.Node.Dir {
		return &appYaml, errors.New("should not be dir")
	}
	if err = YAMLDecode(r.Node.Value, &appYaml); err != nil {
		return &appYaml, err
	}
	return &appYaml, nil
}

func (self *Application) GetConfigYaml() (*ConfigYaml, error) {
	var configYaml ConfigYaml
	cpath := self.GetYamlPath("config")
	r, err := etcdClient.Get(cpath, false, false)
	if err != nil {
		return &configYaml, err
	}
	if r.Node.Dir {
		return &configYaml, errors.New("should not be dir")
	}
	if err = YAMLDecode(r.Node.Value, &configYaml); err != nil {
		return &configYaml, err
	}
	return &configYaml, nil
}

func (self *Application) UserUid() int {
	return self.User.Id
}

func (self *Application) SetImageAddr(addr string) {
	self.ImageAddr = addr
	db.Update(self)
}

func (self *Application) Containers() []*Container {
	var cs []*Container
	db.QueryTable(new(Container)).Filter("AppId", self.Id).OrderBy("Port").All(&cs)
	return cs
}

func (self *Application) AllVersionHosts() []*Host {
	var rs orm.ParamsList
	var hosts []*Host
	_, err := db.Raw("SELECT distinct(host_id) FROM container WHERE app_name=?", self.Name).ValuesFlat(&rs)
	if err == nil && len(rs) > 0 {
		db.QueryTable(new(Host)).Filter("id__in", rs).All(&hosts)
	}
	return hosts
}
