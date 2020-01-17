package volume_test

import (
	"github.com/swift9/ares-nas/volume"
	"testing"
	"time"
)

func TestNewVolumeDaemon(t *testing.T) {
	vd := volume.NewVolumeDaemon(&volume.NasVolumeDaemonOption{
		CheckInterval: 30 * time.Second,
		LoadNasVolumeMountOptions: func() []volume.NasVolumeMountOption {
			return []volume.NasVolumeMountOption{
				volume.NasVolumeMountOption{
					Name:   "Y",
					NodeIp: "10.20.0.13",
					Nas: []volume.NasVolumeDef{
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
