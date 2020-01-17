package volume

import (
	"github.com/swift9/ares-nas/utils"
	"sync"
	"time"
)

type NasVolumeDef struct {
	FsType   string
	Server   string
	Path     string
	User     string
	Password string
	MountTo  string
	Group    string
}

type NasVolumeMountOption struct {
	Name   string
	NodeIp string
	Nas    []NasVolumeDef
}

type NasVolume interface {
	IsMounted() bool
	Mount() error
	UMount() error
	IsHealth() bool
	GetServer() string
}

type NasVolumeProxy struct {
	Name              string
	Node              string
	VolumeMountOption NasVolumeMountOption
	DefaultIndex      int
	Index             int
	Volumes           []NasVolume
	VolumesMd5        string
}

func (vp *NasVolumeProxy) Mount() error {
	volume := vp.GetVolume()
	err := volume.Mount()
	return err
}

func (vp *NasVolumeProxy) MountNextVolume() error {
	vp.UMount()
	volume := vp.SelectNextVolume()
	err := volume.Mount()
	return err
}

func (vp *NasVolumeProxy) ResumeDefaultMount() error {
	vp.UMount()
	vp.Index = -1
	return vp.Mount()
}

func (vp *NasVolumeProxy) UMount() error {
	volume := vp.GetVolume()
	return volume.UMount()
}

func (vp *NasVolumeProxy) IsMounted() bool {
	return vp.GetVolume().IsMounted()
}

func (vp *NasVolumeProxy) IsHealth() bool {
	return vp.GetVolume().IsHealth()
}

func (vp *NasVolumeProxy) GetServer() string {
	return vp.GetVolume().GetServer()
}

func (vp *NasVolumeProxy) SelectNextVolume() NasVolume {
	if vp.Index == -1 {
		vp.Index = vp.GetDefaultIndex()
	} else {
		vp.Index = vp.Index + 1
	}
	return vp.Volumes[vp.Index%len(vp.Volumes)]
}

func (vp *NasVolumeProxy) GetVolume() NasVolume {
	if vp.Index == -1 {
		vp.Index = vp.GetDefaultIndex()
	}
	return vp.Volumes[vp.Index%len(vp.Volumes)]
}

func (vp *NasVolumeProxy) GetDefaultVolume() NasVolume {
	return vp.Volumes[vp.GetDefaultIndex()%len(vp.Volumes)]
}

func (vp *NasVolumeProxy) GetDefaultIndex() int {
	vp.DefaultIndex = utils.GetLastNumberOfIp(vp.Node)
	return vp.DefaultIndex
}

func newNasVolumeProxy(volumeMountOption NasVolumeMountOption, log Logger) *NasVolumeProxy {
	volumesMd5 := utils.ObjectToMd5(volumeMountOption)
	volumes := make([]NasVolume, 0)
	for _, nas := range volumeMountOption.Nas {
		func(s NasVolumeDef) {
			volumes = append(volumes, NewNasVolumeInstance(s, log))
		}(nas)
	}
	volumeProxy := NasVolumeProxy{
		Name:         volumeMountOption.Name,
		Node:         volumeMountOption.NodeIp,
		VolumesMd5:   volumesMd5,
		Volumes:      volumes,
		Index:        -1,
		DefaultIndex: 0,
	}
	return &volumeProxy
}

type Logger interface {
	Info(args ...interface{})
	Error(args ...interface{})
}

type emptyLogger struct {
}

func (l *emptyLogger) Info(args ...interface{}) {
}

func (l *emptyLogger) Error(args ...interface{}) {
}

type NasVolumeDaemon struct {
	volumeDefsMd5      string
	volumeProxyMap     sync.Map
	volumeDaemonOption *NasVolumeDaemonOption
}

type NasVolumeDaemonOption struct {
	CheckInterval             time.Duration
	Log                       Logger
	LoadNasVolumeMountOptions func() []NasVolumeMountOption
}

func NewVolumeDaemon(opt *NasVolumeDaemonOption) *NasVolumeDaemon {
	if opt.Log == nil {
		opt.Log = &emptyLogger{}
	}
	vd := &NasVolumeDaemon{
		volumeProxyMap:     sync.Map{},
		volumeDaemonOption: opt,
	}
	return vd
}

func (vd *NasVolumeDaemon) Start() {
	defer func() {
		if err := recover(); err != nil {
			vd.GetLog().Error("NasVolumeDaemon Start error ", err)
		}
	}()

	for {
		vd.autoMount()
		isMounted := true
		vd.volumeProxyMap.Range(func(key, value interface{}) bool {
			volumeProxy, _ := value.(*NasVolumeProxy)
			if volumeProxy.IsHealth() && volumeProxy.IsMounted() {
				return true
			}
			isMounted = false
			return false
		})
		if isMounted {
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}
	go func() {
		for {
			time.Sleep(vd.GetCheckInterval())
			vd.autoMount()
		}
	}()
}

func (vd *NasVolumeDaemon) GetLog() Logger {
	return vd.volumeDaemonOption.Log
}

func (vd *NasVolumeDaemon) GetCheckInterval() time.Duration {
	defaultCheckInterval := 5 * time.Second
	if defaultCheckInterval.Seconds() > vd.volumeDaemonOption.CheckInterval.Seconds() {
		return defaultCheckInterval
	}
	return vd.volumeDaemonOption.CheckInterval
}

func (vd *NasVolumeDaemon) autoMount() {
	defer func() {
		if err := recover(); err != nil {
			vd.GetLog().Error("NasVolumeDaemon autoMount error ", err)
		}
	}()
	volumeMountOptions, _ := vd.refreshVolumeMountOptions()
	if len(volumeMountOptions) > 0 {
		vd.onVolumeMountOptionsChange(volumeMountOptions)
	}
	vd.volumeProxyMap.Range(func(key, value interface{}) bool {
		volumeProxy, _ := value.(*NasVolumeProxy)
		if volumeProxy.IsHealth() {
			if !volumeProxy.IsMounted() {
				volumeProxy.Mount()
				return true
			}
		} else {
			volumeProxy.MountNextVolume()
			return true
		}
		if volumeProxy.Index != volumeProxy.DefaultIndex && volumeProxy.GetDefaultVolume().IsHealth() {
			volumeProxy.ResumeDefaultMount()
		}
		return true
	})
}

func (vd *NasVolumeDaemon) refreshVolumeMountOptions() ([]NasVolumeMountOption, bool) {
	volumeDefs := vd.volumeDaemonOption.LoadNasVolumeMountOptions()
	if len(volumeDefs) == 0 {
		return nil, false
	}
	md5 := utils.ObjectToMd5(volumeDefs)
	if md5 == "" || md5 == vd.volumeDefsMd5 {
		return nil, false
	}
	return volumeDefs, true
}

func (vd *NasVolumeDaemon) onVolumeMountOptionsChange(volumeMountOptions []NasVolumeMountOption) error {
	volumeProxyMap := sync.Map{}
	for _, volumeDef := range volumeMountOptions {
		volumeProxy := newNasVolumeProxy(volumeDef, vd.GetLog())
		volumeProxyMap.Store(volumeDef.Name, volumeProxy)
	}

	// 删除已移除的磁盘，保留老盘
	vd.volumeProxyMap.Range(func(key, value interface{}) bool {
		oldVolumeProxy, _ := value.(*NasVolumeProxy)
		var newVolumeProxyInterface interface{}
		var newVolumeProxy *NasVolumeProxy
		var ok = false
		if newVolumeProxyInterface, ok = volumeProxyMap.Load(key); !ok {
			oldVolumeProxy.UMount()
			return true
		}
		newVolumeProxy, _ = newVolumeProxyInterface.(*NasVolumeProxy)
		if newVolumeProxy.VolumesMd5 == oldVolumeProxy.VolumesMd5 {
			volumeProxyMap.Store(key, oldVolumeProxy)
		}
		return true
	})

	vd.volumeProxyMap = volumeProxyMap
	return nil
}
