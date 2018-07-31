package main

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

// Scalers - all monitored apps
// TODO GET RID OF THIS - should be attached to Repo (or Manager, as it will be called)
type Scalers []Scaler

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

// MVP: need to expand so we can log more information
func (scaler *Scaler) autoscaleCpuMem(cpu, mem float64) (int) {
	cpuDirection := getDirection("cpu", cpu, scaler.MinCPU, scaler.MaxCPU)
	memDirection := getDirection("mem", mem, scaler.MinMem, scaler.MaxMem)
	direction := 0
	switch method := scaler.Method; method {
	case "cpu":
		direction = cpuDirection
	case "mem":
		direction = memDirection
	case "and":
		if cpuDirection == memDirection {
			direction = cpuDirection
		}
	case "or":
		if (cpuDirection + memDirection) > 0 {
			direction = 1
		} else if (cpuDirection + memDirection) < 0 {
			direction = -1
		}
	}

	return direction
}

// TODO actually output value of direction
func getDirection(label string, value float64, min float64, max float64) (int) {
	if value > max {
		log.Infof("%s value [%f] higher than max [%f]", label, value, max)
		return 1
	} else if value < min {
		log.Infof("%s value [%f] lower than min [%f]", label, value, min)
		return -1
	} else {
		log.Infof("%s value [%f] between min [%f] and max [%f]", label, value, min)
		return 0
	}
}

//AutoScale track and scale apps
func (scaler *Scaler) AutoScale(cpu, mem float64, st *scalerState, mApp MarathonApp) {
	direction := scaler.autoscaleCpuMem(cpu, mem)
	// sig := generateSignal(cpu, mem, scaler)
	if direction == 1 {
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
	} else if direction == -1 {
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

	} else {
		st.coolDown = 0
		st.warmUp = 0
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
