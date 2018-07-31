package main

// Scalers - all monitored apps
// TODO GET RID OF THIS
type Scalers []Scaler

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

//ScaleSignal describes a scale proposal
type ScaleSignal struct {
	Scale scaleDirection
}

// This needs to be turned into an int: -1, 0, 1
type scaleDirection struct {
	up   bool
	down bool
}

//Scaler struct describing autoscaling policy
type Scaler struct {
	AppID        string  `json:"app_id"`
	MaxCPU       float64 `json:"max_cpu"`
	MinCPU       float64 `json:"min_cpu"`
	MaxMem       float64 `json:"max_mem"`
	MinMem       float64 `json:"min_mem"`
	Method       string  `json:"method"`
	ScaleFactor  int     `json:"scale_factor"`
	MaxInstances int     `json:"max_instances"`
	MinInstances int     `json:"min_instances"`
	WarmUp       int     `json:"warm_up"`
	CoolDown     int     `json:"cool_down"`
	Interval     int     `json:"interval"`
}

type scalerState struct {
	warmUp   int
	coolDown int
}

//StartMonitor starts a ticker goroutine
func (scaler *Scaler) StartMonitor() {
	tickers[scaler.AppID] = time.NewTicker(time.Second * time.Duration(scaler.Interval))
	go scaler.doMonitor()
}

//doMonitor will be storing the intermediate state of the app metrics
func (scaler *Scaler) doMonitor() {
	as := scalerState{0, 0}
	var cpu, mem float64
	for range tickers[scaler.AppID].C {
		if !client.AppExists(scaler) {
			log.Warningf("%s not found in /service/marathon/v2/app", scaler.AppID)
			continue
		}
		marathonApp := client.GetMarathonApp(scaler.AppID)
		if marathonApp.App.Instances == 0 {
			log.Warningf("%s suspended, skipping monitoring cycle", marathonApp.App.ID)
			continue
		}
		if !scaler.EnsureMinMaxInstances(marathonApp) {
			continue
		}
		cpu, mem = scaler.getCPUMem(marathonApp)
		log.Infof("app:%s cpu:%f, mem:%f", scaler.AppID, cpu, mem)
		scaler.AutoScale(cpu, mem, &as, marathonApp)
	}
}

//StopMonitor stops the ticker associated with the given app
func (scaler *Scaler) StopMonitor() {
	tickers[scaler.AppID].Stop()
}

// TODO this doesn't need to be attached to Scaler, I don't think
func (scaler *Scaler) getCPUMem(marathonApp MarathonApp) (float64, float64) {
	var (
		stats1, stats2               TaskStats
		cpu, cpu1, cpu2, cpuD, timeD float64
		mem                          float64
	)
	marathonApp.FilterNonRunningTasks()
	for _, task := range marathonApp.App.Tasks {
		stats1 = client.GetTaskStats(task.ID, task.SlaveID)
		//TODO: implement a trailing data structure here
		time.Sleep(time.Second * 1)
		stats2 = client.GetTaskStats(task.ID, task.SlaveID)

		cpu1 = stats1.Statistics.CpusSystemTimeSecs + stats1.Statistics.CpusUserTimeSecs
		cpu2 = stats2.Statistics.CpusSystemTimeSecs + stats2.Statistics.CpusUserTimeSecs
		cpuD = cpu2 - cpu1
		timeD = stats2.Statistics.Timestamp - stats1.Statistics.Timestamp
		cpu = cpu + (cpuD / timeD)
		mem = mem + (stats1.Statistics.MemRssBytes / stats1.Statistics.MemLimitBytes)
	}
	cpu = cpu / float64(len(marathonApp.App.Tasks)) * 100
	mem = mem / float64(len(marathonApp.App.Tasks)) * 100
	return cpu, mem
}


//generateSignal given cpu and mem values, return a scale proposal
func generateSignal(cpu, mem float64, scaler *Scaler) ScaleSignal {
	result := ScaleSignal{}
	cpuDown := (cpu <= scaler.MinCPU)
	cpuUp := (cpu > scaler.MaxCPU)
	memDown := (mem <= scaler.MinMem)
	memUp := (mem > scaler.MinMem)
	switch method := scaler.Method; method {
	case "cpu":
		result.Scale.up = cpuUp
		result.Scale.down = cpuDown
	case "mem":
		result.Scale.up = memUp
		result.Scale.down = memDown
	case "and":
		result.Scale.up = cpuUp && memUp
		result.Scale.down = cpuDown && memDown
	case "or":
		result.Scale.up = cpuUp || memUp
		result.Scale.down = cpuDown || memDown
	default:
		log.Errorf("method should be cpu|mem|and|or: %s", method)
		log.Panicln("Invalid scaling parameter method.")
	}
	if result.Scale.up && result.Scale.down {
		log.Warnf("Scale up and scale down signal generated, defaulting to no operation. %+v", result)
		result.Scale.up = false
		result.Scale.down = false
	}

	return result
}

//AutoScale track and scale apps
func (scaler *Scaler) AutoScale(cpu, mem float64, st *scalerState, mApp MarathonApp) {
	sig := generateSignal(cpu, mem, scaler)
	if !sig.Scale.down && !sig.Scale.up {
		st.coolDown = 0
		st.warmUp = 0
	} else {
		if sig.Scale.up {
			if mApp.App.Instances < scaler.MaxInstances {
				st.warmUp++
				if st.warmUp >= scaler.WarmUp {
					log.Infof("%s scale up triggered with %d of %d signals of %s",
						scaler.AppID, st.warmUp, scaler.WarmUp, scaler.Method)
					scaler.doScale(mApp, scaler.ScaleFactor)
					st.warmUp = 0
				} else {
					log.Infof("%s warming up %s(%d of %d)",
						scaler.AppID, scaler.Method, st.warmUp, scaler.WarmUp)
				}
			} else {
				log.Infof("%s reached max instances %d", scaler.AppID, scaler.MaxInstances)
			}
		}
		if sig.Scale.down {
			if mApp.App.Instances > scaler.MinInstances {
				st.coolDown++
				if st.coolDown >= scaler.CoolDown {
					log.Infof("%s scale down triggered with %d of %d signals of %s",
						scaler.AppID, st.coolDown, scaler.CoolDown, scaler.Method)
					scaler.doScale(mApp, -scaler.ScaleFactor)
					st.coolDown = 0
				} else {
					log.Infof("%s cooling down %s(%d of %d)",
						scaler.AppID, scaler.Method, st.coolDown, scaler.CoolDown)
				}
			} else {
				log.Infof("%s reached min instances %d", scaler.AppID, scaler.MinInstances)
			}
		}
	}

}

//EnsureMinMaxInstances scales up or down to get within Min-Max instances
func (scaler *Scaler) EnsureMinMaxInstances(mApp MarathonApp) bool {
	diff := 0
	if mApp.App.Instances < scaler.MinInstances {
		diff = scaler.MinInstances - mApp.App.Instances
		log.Infof("%s will be scaled up by %d to reach minimum instances of %d",
			scaler.AppID, diff, scaler.MinInstances)
		scaler.doScale(mApp, diff)
	} else if mApp.App.Instances > scaler.MaxInstances {
		diff = scaler.MaxInstances - mApp.App.Instances
		log.Infof("%s will be scaled down by %d to reach maximum instances of %d",
			scaler.AppID, diff, scaler.MaxInstances)
		scaler.doScale(mApp, diff)
	}
	return diff == 0
}

func (scaler *Scaler) doScale(mApp MarathonApp, instances int) {
	target := mApp.App.Instances + instances
	if target > scaler.MaxInstances {
		target = scaler.MaxInstances
	} else if target < scaler.MinInstances {
		target = scaler.MinInstances
	}
	log.Infof("Scaling %s to %d instances", scaler.AppID, target)
	client.ScaleMarathonApp(scaler.AppID, target)
}
