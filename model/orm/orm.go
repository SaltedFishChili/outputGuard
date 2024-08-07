package orm

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	. "outputGuard/logger"

	"gopkg.in/yaml.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type CrawlerProxy struct {
	ID         uint      `gorm:"primaryKey"`
	Types      string    `gorm:"column:types"`
	IP         string    `gorm:"column:ip"`
	Name       string    `gorm:"column:name"`
	IsNoDel    bool      `gorm:"column:is_no_del"`
	IsLocalNet bool      `gorm:"column:is_local_net"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// var Ormer *ORM = NewORM()

type ORM struct {
	db *gorm.DB
}

type config struct {
	DbUser     string `yaml:"db_user"`
	DbPassword string `yaml:"db_password"`
	DbServer   string `yaml:"db_server"`
	DbPort     string `yaml:"db_port"`
	DbName     string `yaml:"db_name"`
}

func LoadConfig() (*config, error) {
	var configPath string
	flag.StringVar(&configPath, "server-conf-path", "", "设置server配置文件路径")
	flag.Parse()

	if configPath == "" {
		return nil, fmt.Errorf("配置文件路径不能为空")
	}
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件: %v", err)
	}

	var config config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("无法解析配置文件: %v", err)
	}

	return &config, nil
}

func NewORM() *ORM {
	config, err := LoadConfig()
	if err != nil {
		Logger.Panic(fmt.Sprintf("加载server配置文件失败:%s", err.Error()))
		return nil
	}
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", config.DbUser, config.DbPassword, config.DbServer, config.DbPort, config.DbName)
	// 创建 Gorm 的 DB 对象
	rootDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN: rootDSN,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		Logger.Panic(fmt.Sprintf("创建数据库失败:%s", err.Error()))
		return nil
	}

	// 设置连接池参数
	sqlDB, err := rootDB.DB()
	if err != nil {
		Logger.Panic(fmt.Sprintf("设置数据库连接池参数失败:%s", err.Error()))
		return nil
	}

	sqlDB.SetMaxIdleConns(1000)                // 设置最大空闲连接数
	sqlDB.SetMaxOpenConns(1000)                // 设置最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Second * 10) // 设置连接的最大存活时间
	sqlDB.SetConnMaxIdleTime(time.Second * 10) // 设置连接的最大空闲时间

	migrator := rootDB.Migrator()
	if !migrator.HasTable(&CrawlerProxy{}) {
		if err := rootDB.AutoMigrate(&CrawlerProxy{}); err != nil {
			Logger.Panic(fmt.Sprintf("数据库migrator失败:%s", err.Error()))
			return nil
		}
	}

	return &ORM{db: rootDB}
}

func (orm *ORM) Add(Types, ip, Name string, CreatedAt time.Time, isNoDel, isLocalNet bool) error {
	ipExists, err := orm.Query(ip)
	if err != nil {
		return err
	}
	if ipExists {
		Logger.Info(fmt.Sprintf("ip %s 已存在,不再添加到数据库", ip))
		return nil
	}
	if err := orm.db.Create(&CrawlerProxy{
		IP:         ip,
		Types:      Types,
		CreatedAt:  CreatedAt,
		Name:       Name,
		IsNoDel:    isNoDel,
		IsLocalNet: isLocalNet,
	}).Error; err != nil {
		return err
	}
	return nil
}

func (orm *ORM) Query(ip string) (bool, error) {

	var res CrawlerProxy
	if err := orm.db.Where("ip = ?", ip).Find(&res).Error; err != nil {
		return false, err
	}
	if res.ID == 0 {
		return false, nil
	}
	return true, nil
}

func (orm *ORM) QueryNoDel(ip string) (bool, error) {

	var res CrawlerProxy
	if err := orm.db.Where("ip = ? AND is_no_del = ?", ip, true).Find(&res).Error; err != nil {
		return false, err
	}
	if res.ID == 0 {
		return false, nil
	}
	return true, nil
}

func (orm *ORM) Del(ip string) error {
	noDel, err := orm.QueryNoDel(ip)
	if err != nil {
		return err
	}
	if noDel {
		Logger.Info(fmt.Sprintf("IP %s 为不能删除IP!", ip))
		return nil
	}
	if err := orm.db.Where("ip = ?", ip).Delete(&CrawlerProxy{}).Error; err != nil {
		return err
	}
	return nil
}

func (orm *ORM) QueryAll() ([]CrawlerProxy, error) {
	var res []CrawlerProxy
	if err := orm.db.Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (orm *ORM) QueryUniqueDomainNames() ([]string, error) {
	var res []string
	if err := orm.db.Model(&CrawlerProxy{}).Where("types = ? AND is_no_del = ?", "domain", false).Distinct().Pluck("name", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (orm *ORM) QueryNoDelBeforeTime(beforeTime time.Time) ([]string, error) {
	var res []string
	if err := orm.db.Model(&CrawlerProxy{}).Where("is_no_del = ? AND created_at <= ?", false, beforeTime).Pluck("ip", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}
