package nas_test

import (
	nas "github.com/swift9/ares-nas"
	"testing"
	"time"
)

func TestNewVolumeDaemon(t *testing.T) {
	vd := nas.NewVolumeDaemon(&nas.AresNasVolumeDaemonOption{
		CheckInterval: 30 * time.Second,
		LoadNasVolumeMountOptions: func() []nas.AresNasVolumeMountOption {
			return []nas.AresNasVolumeMountOption{
				nas.AresNasVolumeMountOption{
					Name:   "Y",
					NodeIp: "10.20.0.13",
					Nas: []nas.VolumeDef{
						{
							FsType:   "smb",
							Server:   "10.20.0.100",
							Path:     "gong2",
							User:     "Administrator",
							Password: "123456",
							MountTo:  "Y:",
						},
						{
							FsType:   "smb",
							Server:   "192.168.1.100",
							Path:     "gong",
							User:     "Administrator",
							Password: "123456",
							MountTo:  "Y:",
						},
					},
				},
			}
		},
	})
	vd.Start()
	for {
		time.Sleep(1 * time.Second)
	}

}
