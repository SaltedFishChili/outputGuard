package orm

import (
	"fmt"
	"runtime"
	"time"

	. "outputGuard/logger"

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

var Ormer *ORM = NewORM()

type ORM struct {
	db *gorm.DB
}

func NewORM() *ORM {
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", "cmdb", "cmdb", "oss-cmdb-mariadb-master.op.svc.infra.local", "3306", "cmdb")
	switch runtime.GOOS {
	case "darwin":
		rootDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", "root", "lagou_lagou_LAGOU_!@#$1-1", "10.240.32.201", "3306", "cmdb")
	}
	// 创建 Gorm 的 DB 对象
	rootDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN: rootDSN,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		Logger.Error(fmt.Sprintf("创建数据库失败:%s", err.Error()))
		return nil
	}

	// 设置连接池参数
	sqlDB, err := rootDB.DB()
	if err != nil {
		Logger.Error(fmt.Sprintf("设置数据库连接池参数失败:%s", err.Error()))
		return nil
	}

	sqlDB.SetMaxIdleConns(100)               // 设置最大空闲连接数
	sqlDB.SetMaxOpenConns(100)               // 设置最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Hour * 24) // 设置连接的最大存活时间

	migrator := rootDB.Migrator()
	if !migrator.HasTable(&CrawlerProxy{}) {
		if err := rootDB.AutoMigrate(&CrawlerProxy{}); err != nil {
			Logger.Panic(err.Error())
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
