package nas

import (
	"github.com/swift9/ares-nas/utils"
	"sync"
	"time"
)

type VolumeDef struct {
	FsType   string
	Server   string
	Path     string
	User     string
	Password string
	MountTo  string
}

type VolumeMountOption struct {
	Name   string
	NodeIp string
	Nas    []VolumeDef
}

type Volume interface {
	IsMounted() bool
	Mount() error
	UMount() error
	IsHealth() bool
	GetServer() string
}

type AresNasVolumeProxy struct {
	Name              string
	Node              string
	VolumeMountOption VolumeMountOption
	DefaultIndex      int
	Index             int
	Volumes           []Volume
	VolumesMd5        string
}

func (vp *AresNasVolumeProxy) Mount() error {
	volume := vp.GetVolume()
	err := volume.Mount()
	return err
}

func (vp *AresNasVolumeProxy) MountNextVolume() error {
	vp.UMount()
	volume := vp.SelectNextVolume()
	err := volume.Mount()
	return err
}

func (vp *AresNasVolumeProxy) ResumeDefaultMount() error {
	vp.UMount()
	vp.Index = -1
	return vp.Mount()
}

func (vp *AresNasVolumeProxy) UMount() error {
	volume := vp.GetVolume()
	return volume.UMount()
}

func (vp *AresNasVolumeProxy) IsMounted() bool {
	return vp.GetVolume().IsMounted()
}

func (vp *AresNasVolumeProxy) IsHealth() bool {
	return vp.GetVolume().IsHealth()
}

func (vp *AresNasVolumeProxy) GetServer() string {
	return vp.GetVolume().GetServer()
}

func (vp *AresNasVolumeProxy) SelectNextVolume() Volume {
	if vp.Index == -1 {
		vp.Index = vp.GetDefaultIndex()
	} else {
		vp.Index = vp.Index + 1
	}
	return vp.Volumes[vp.Index%len(vp.Volumes)]
}

func (vp *AresNasVolumeProxy) GetVolume() Volume {
	if vp.Index == -1 {
		vp.Index = vp.GetDefaultIndex()
	}
	return vp.Volumes[vp.Index%len(vp.Volumes)]
}

func (vp *AresNasVolumeProxy) GetDefaultVolume() Volume {
	return vp.Volumes[vp.GetDefaultIndex()%len(vp.Volumes)]
}

func (vp *AresNasVolumeProxy) GetDefaultIndex() int {
	vp.DefaultIndex = utils.GetLastNumberOfIp(vp.Node)
	return vp.DefaultIndex
}

func newAresNasVolumeProxy(volumeMountOption VolumeMountOption, log Logger) *AresNasVolumeProxy {
	volumesMd5 := utils.ObjectToMd5(volumeMountOption)
	volumes := make([]Volume, 0)
	for _, nas := range volumeMountOption.Nas {
		func(s VolumeDef) {
			volumes = append(volumes, NewNasVolumeInstance(s, log))
		}(nas)
	}
	volumeProxy := AresNasVolumeProxy{
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

type VolumeDaemon struct {
	volumeDefsMd5          string
	volumeProxyMap         sync.Map
	log                    Logger
	loadVolumeMountOptions func() []VolumeMountOption
}

type VolumeDaemonOption struct {
	Log                    Logger
	LoadVolumeMountOptions func() []VolumeMountOption
}

func NewVolumeDaemon(opt *VolumeDaemonOption) *VolumeDaemon {
	if opt.Log == nil {
		opt.Log = &emptyLogger{}
	}
	vd := &VolumeDaemon{
		volumeProxyMap:         sync.Map{},
		log:                    opt.Log,
		loadVolumeMountOptions: opt.LoadVolumeMountOptions,
	}
	return vd
}

func (vd *VolumeDaemon) Start() {
	defer func() {
		if err := recover(); err != nil {
			vd.log.Error("VolumeDaemon Start error ", err)
		}
	}()

	for {
		vd.autoMount()
		isMounted := true
		vd.volumeProxyMap.Range(func(key, value interface{}) bool {
			volumeProxy, _ := value.(*AresNasVolumeProxy)
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
			time.Sleep(5 * time.Second)
			vd.autoMount()
		}
	}()
}

func (vd *VolumeDaemon) autoMount() {
	defer func() {
		if err := recover(); err != nil {
			vd.log.Error("VolumeDaemon autoMount error ", err)
		}
	}()
	volumeMountOptions, _ := vd.refreshVolumeMountOptions()
	if len(volumeMountOptions) > 0 {
		vd.onVolumeMountOptionsChange(volumeMountOptions)
	}
	vd.volumeProxyMap.Range(func(key, value interface{}) bool {
		volumeProxy, _ := value.(*AresNasVolumeProxy)
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

func (vd *VolumeDaemon) refreshVolumeMountOptions() ([]VolumeMountOption, bool) {
	volumeDefs := vd.loadVolumeMountOptions()
	if len(volumeDefs) == 0 {
		return nil, false
	}
	md5 := utils.ObjectToMd5(volumeDefs)
	if md5 == "" || md5 == vd.volumeDefsMd5 {
		return nil, false
	}
	return volumeDefs, true
}

func (vd *VolumeDaemon) onVolumeMountOptionsChange(volumeMountOptions []VolumeMountOption) error {
	volumeProxyMap := sync.Map{}
	for _, volumeDef := range volumeMountOptions {
		volumeProxy := newAresNasVolumeProxy(volumeDef, vd.log)
		volumeProxyMap.Store(volumeDef.Name, volumeProxy)
	}

	// 删除已移除的磁盘，保留老盘
	vd.volumeProxyMap.Range(func(key, value interface{}) bool {
		oldVolumeProxy, _ := value.(*AresNasVolumeProxy)
		var newVolumeProxyInterface interface{}
		var newVolumeProxy *AresNasVolumeProxy
		var ok = false
		if newVolumeProxyInterface, ok = volumeProxyMap.Load(key); !ok {
			oldVolumeProxy.UMount()
			return true
		}
		newVolumeProxy, _ = newVolumeProxyInterface.(*AresNasVolumeProxy)
		if newVolumeProxy.VolumesMd5 == oldVolumeProxy.VolumesMd5 {
			volumeProxyMap.Store(key, oldVolumeProxy)
		}
		return true
	})

	vd.volumeProxyMap = volumeProxyMap
	return nil
}
