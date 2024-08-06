package service

import (
	"outputGuard/global"

	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

func NewIpts() (IptableRules, error) {
	irs := IptableRules{}
	ipt, err := iptables.New()
	if err != nil {
		return irs, err
	}
	irs.Ipt = ipt
	irs.Table = "filter"
	return irs, nil

}

type IptableRules struct {
	Ipt       *iptables.IPTables
	ChainName string
	Table     string
	// Ip        string
	Packets int
	Bytes   int
}

func (ir IptableRules) AddMasqueradeRule(ip string) error {
	ruleSpec := []string{"-s", "0.0.0.0/0", "-d", ip, "-j", "MASQUERADE"}

	if err := ir.Ipt.InsertUnique("nat", "POSTROUTING", 1, ruleSpec...); err != nil {
		return err
	}

	return nil
}

func (ir IptableRules) DeleteMasqueradeRule(ip string) error {

	ruleSpec := []string{"-s", "0.0.0.0/0", "-d", ip, "-j", "MASQUERADE"}
	if err := ir.Ipt.DeleteIfExists("nat", "POSTROUTING", ruleSpec...); err != nil {
		return err
	}

	return nil
}

func (ir IptableRules) AddForwordRule(ip string) error {
	ruleIn := []string{"-s", ip, "-j", "ACCEPT"}
	if err := ir.Ipt.InsertUnique(ir.Table, "FORWARD", 1, ruleIn...); err != nil {
		return err
	}
	ruleOut := []string{"-d", ip, "-j", "ACCEPT"}
	if err := ir.Ipt.InsertUnique(ir.Table, "FORWARD", 1, ruleOut...); err != nil {
		return err
	}
	return nil
}

func (ir IptableRules) DeleteForwordRule(ip string) error {
	ruleIn := []string{"-s", ip, "-j", "ACCEPT"}
	if err := ir.Ipt.DeleteIfExists(ir.Table, "FORWARD", ruleIn...); err != nil {
		return err
	}
	ruleOut := []string{"-d", ip, "-j", "ACCEPT"}
	if err := ir.Ipt.DeleteIfExists(ir.Table, "FORWARD", ruleOut...); err != nil {

		return nil
	}
	return nil
}

func (ir IptableRules) AddAccept(ip string) error {
	ruleSpec := []string{"-s", ip, "-j", "ACCEPT"}

	if err := ir.Ipt.InsertUnique(ir.Table, "INPUT", 1, ruleSpec...); err != nil {
		return err
	}

	if err := ir.Ipt.InsertUnique(ir.Table, "OUTPUT", 1, ruleSpec...); err != nil {
		return err
	}
	return nil
}

func (ir IptableRules) DeleteAccept(ip string) error {
	ruleSpec := []string{"-s", ip, "-j", "ACCEPT"}
	if err := ir.Ipt.DeleteIfExists(ir.Table, "INPUT", ruleSpec...); err != nil {
		return err
	}
	if err := ir.Ipt.DeleteIfExists(ir.Table, "OUTPUT", ruleSpec...); err != nil {
		return err
	}

	return nil
}

func (ir IptableRules) GetRules() ([]string, error) {
	inputRules, err := ir.Ipt.List(ir.Table, "INPUT")
	if err != nil {
		return nil, err
	}

	outputRules, err := ir.Ipt.List(ir.Table, "OUTPUT")
	if err != nil {
		return nil, err
	}

	return append(inputRules, outputRules...), nil
}

func (ir IptableRules) extractIPsFromRule(rule string) string {
	re := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?:\/\d{1,2})?\b`)
	matchs := re.FindStringSubmatch(rule)
	if len(matchs) == 0 {
		return ""
	}
	return matchs[0]
}
func (ir *IptableRules) Count(table, chain string) error {
	rules, err := ir.Ipt.ListWithCounters(table, chain)
	if err != nil {
		return err
	}
	uniqueRules := make(map[string]global.ExporterData)
	var errs error
	for _, rule := range rules {
		ge, err := ir.parseRule(rule)
		if err != nil {
			errs = err
			continue
		}
		if ge.Ip == "" {
			continue
		}
		key := fmt.Sprintf("%s_%s", ge.ChainName, ge.Ip)
		existingRule, exists := uniqueRules[key]
		// 去除重复的规则
		// 监控暴露规则命中的总数
		if exists {
			ge.Packets += existingRule.Packets
			ge.Bytes += existingRule.Bytes
			uniqueRules[key] = ge
			continue
		}
		uniqueRules[key] = ge

	}

	for _, ge := range uniqueRules {
		global.ExporterDatas <- ge
	}

	return errs
}

func (ir IptableRules) parseRule(rule string) (global.ExporterData, error) {
	var ge global.ExporterData
	parts := strings.Fields(rule)

	if len(parts) < 8 {
		return ge, nil
	}
	var chainType string
	direction := parts[2]
	chainIp := parts[3]
	t := ""
	switch {
	case direction == "-d":
		chainType = "POSTROUTING"
		t = "OUTPUT"
	case direction == "-s":
		chainType = "FORWARD"
		t = "INPUT"
	}

	chainPacketsFloat, ok := toFloat64(parts[5])
	if !ok {
		return ge, fmt.Errorf("%s尝试转换为float64 chainPacketsFloat失败", parts[5])
	}

	chainBytesFloat, ok := toFloat64(parts[6])
	if !ok {
		return ge, fmt.Errorf("%s尝试转换为float64 chainBytesFloat失败", parts[6])
	}

	ge.ChainName = chainType
	ge.Bytes = chainBytesFloat
	ge.Packets = chainPacketsFloat
	ge.Ip = chainIp
	ge.Direction = t
	ge.Hostname, _ = os.Hostname()
	return ge, nil
}

func toFloat64(value string) (float64, bool) {
	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return result, true
}

func (ir IptableRules) Cache() ([]string, error) {
	ipList := make([]string, 0)
	iptablesRules, err := ir.GetRules()
	if err != nil {
		return ipList, err
	}
	for _, rule := range iptablesRules {
		ruleIp := ir.extractIPsFromRule(rule)
		if ruleIp != "" {
			ipList = append(ipList, ruleIp)
		}
	}
	return ipList, nil
}

func (ir IptableRules) AddDropAll() error {
	dropRule := []string{"-j", "DROP"}

	if err := ir.Ipt.AppendUnique(ir.Table, "INPUT", dropRule...); err != nil {
		return err
	}

	if err := ir.Ipt.AppendUnique(ir.Table, "OUTPUT", dropRule...); err != nil {
		return err
	}

	return nil
}

func (ir IptableRules) CheckForwardAcceptRule() error {
	forwardRules, err := ir.Ipt.List(ir.Table, "FORWARD")
	if err != nil {
		return err
	}

	for _, rule := range forwardRules {
		if strings.Contains(rule, "-P FORWARD ACCEPT") {
			return nil
		}
	}

	forwardAcceptRule := []string{"-P", "FORWARD", "ACCEPT"}
	if err := ir.Ipt.AppendUnique(ir.Table, "FORWARD", forwardAcceptRule...); err != nil {
		return err
	}

	return nil
}

func (ir IptableRules) InitAddLocalNet() error {
	localNet := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"255.255.255.255/32",
	}
	for _, local := range localNet {
		if err := ir.AddAccept(local); err != nil {
			return err
		}
	}
	return nil
}
