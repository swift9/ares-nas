package volume

import (
	"errors"
	"github.com/swift9/ares-nas/utils"
	"net"
	"os"
	sysRuntime "runtime"
	"strings"
	"time"
)

func NewNasVolumeInstance(nasVolume NasVolumeDef, log Logger) NasVolume {
	if nasVolume.FsType == "smb" {
		smb := SmbVolume{
			NasVolumeDef: nasVolume,
			Mounted:      false,
			log:          log,
		}
		return &smb
	}
	log.Error(nasVolume.FsType, " not support")
	return nil
}

type SmbVolume struct {
	NasVolumeDef
	Mounted bool
	log     Logger
}

func (smb *SmbVolume) IsMounted() bool {
	if !smb.Mounted || !smb.IsHealth() {
		return false
	}

	fileInfo, err := os.Stat(smb.MountTo + "/")
	if err != nil || !fileInfo.IsDir() {
		return false
	}

	goos := sysRuntime.GOOS
	if goos == "windows" {
		out, _ := utils.Exec("cmd.exe", []string{"/c", "net", "use", smb.MountTo}, 10*time.Second)
		return strings.Contains(out, smb.Server) && strings.Contains(out, smb.MountTo) && strings.Contains(out, "OK")
	}
	out, _ := utils.Exec("df", []string{"-h"}, 10*time.Second)
	return strings.Contains(out, smb.Server) && strings.Contains(out, smb.MountTo)
}

func (smb *SmbVolume) Mount() error {
	smb.UMount()
	goos := sysRuntime.GOOS
	switch goos {
	case "windows":
		mountCmd := "net use " + smb.MountTo + " \\\\" + smb.Server + "\\" + smb.Path + " " + smb.Password + " /user:" + smb.User
		smb.log.Info(mountCmd)
		out, err := utils.Exec("cmd.exe", []string{"/c", mountCmd}, 10*time.Second)
		smb.Mounted = true
		smb.log.Info(smb.Server, " is mounted ", out, " error:", err)
	case "linux":
		smb.Path = fixToUnixAbsolutePath(smb.Path)
		smb.MountTo = fixToUnixAbsolutePath(smb.MountTo)
		os.MkdirAll(smb.MountTo, os.ModePerm)
		// mount -t cifs -o username=samba,password=123456 //192.168.1.2/samba /X
		out, err := utils.Exec("mount", []string{"-t", "cifs", "-o", "username=" + smb.User + ",password=" + smb.Password, "//" + smb.Server + smb.Path, smb.MountTo}, 10*time.Second)
		smb.Mounted = true
		smb.log.Info(smb.Server, " is mounted ", out, " error:", err)
	case "darwin":
		smb.Path = fixToUnixAbsolutePath(smb.Path)
		smb.MountTo = fixToUnixAbsolutePath(smb.MountTo)
		os.MkdirAll(smb.MountTo, os.ModePerm)
		// mount -t smbfs //samba:123456@192.168.1.2/samba /X
		out, err := utils.Exec("mount", []string{"-t", "smbfs", "//" + smb.User + ":" + smb.Password + "@" + smb.Server + smb.Path, smb.MountTo}, 10*time.Second)
		smb.Mounted = true
		smb.log.Info(smb.Server, " is mounted ", out, " error:", err)
	default:
		smb.log.Error(goos, " not support")
		return errors.New(goos + " mount is not support")
	}
	if !smb.IsMounted() {
		smb.log.Error("mount fail ", smb.Server, " ", smb.MountTo)
		return errors.New("mount fail")
	}
	return nil
}

func (smb *SmbVolume) UMount() error {
	goos := sysRuntime.GOOS
	var (
		out string
		err error
	)
	switch goos {
	case "windows":
		umountCmd := "net use " + smb.MountTo + " /del /y"
		out, err = utils.Exec("cmd.exe", []string{"/c", umountCmd}, 10*time.Second)
		smb.Mounted = false
	case "darwin":
		smb.Path = fixToUnixAbsolutePath(smb.Path)
		smb.MountTo = fixToUnixAbsolutePath(smb.MountTo)
		os.MkdirAll(smb.MountTo, os.ModePerm)
		out, err = utils.Exec("umount", []string{smb.MountTo}, 10*time.Second)
		smb.Mounted = false
	case "linux":
		smb.Path = fixToUnixAbsolutePath(smb.Path)
		smb.MountTo = fixToUnixAbsolutePath(smb.MountTo)
		os.MkdirAll(smb.MountTo, os.ModePerm)
		out, err = utils.Exec("umount", []string{smb.MountTo}, 10*time.Second)
		smb.Mounted = false
	default:
		smb.log.Error(goos, " unmount not support")
		return errors.New(goos + " unmount is not support")
	}
	smb.log.Info(smb.Server, " is umounted ", out, " error:", err)
	return nil
}

func (smb *SmbVolume) IsHealth() (result bool) {
	defer func() {
		if err := recover(); err != nil {
			smb.log.Error("SmbVolume IsHealth error ", err)
		}
	}()
	result = false
	var (
		con net.Conn
		err error
	)
	if con, err = net.DialTimeout("tcp", smb.Server+":445", 3*time.Second); err == nil {
		smb.log.Info(smb.Server, " is health")
		result = true
	} else {
		smb.log.Info(smb.Server, " is not health")
	}
	if con != nil {
		con.Close()
	}
	return result
}

func (smb *SmbVolume) GetServer() string {
	return smb.Server
}

func fixToUnixAbsolutePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
