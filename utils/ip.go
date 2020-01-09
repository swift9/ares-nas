package utils

import (
	"strconv"
	"strings"
)

// 获取IP最后一位
func GetLastNumberOfIp(ip string) int {
	nums := strings.Split(ip, ".")
	if len(nums) < 4 {
		return 0
	}
	num, _ := strconv.Atoi(nums[3])
	return num
}
