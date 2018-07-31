package main

import (
	log "github.com/Sirupsen/logrus"
)

//ScaleSignal describes a scale proposal
type ScaleSignal struct {
	Scale scaleDirection
}

type scaleDirection struct {
	up   bool
	down bool
}

//generateSignal given cpu and mem values, return a scale proposal
func generateSignal(cpu, mem float64, r *Scaler) ScaleSignal {
	result := ScaleSignal{}
	cpuDown := (cpu <= r.MinCPU)
	cpuUp := (cpu > r.MaxCPU)
	memDown := (mem <= r.MinMem)
	memUp := (mem > r.MinMem)
	switch method := r.Method; method {
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
func (r *Scaler) AutoScale(cpu, mem float64, st *scalerState, mApp MarathonApp) {
	sig := generateSignal(cpu, mem, r)
	if !sig.Scale.down && !sig.Scale.up {
		st.coolDown = 0
		st.warmUp = 0
	} else {
		if sig.Scale.up {
			if mApp.App.Instances < r.MaxInstances {
				st.warmUp++
				if st.warmUp >= r.WarmUp {
					log.Infof("%s scale up triggered with %d of %d signals of %s",
						r.AppID, st.warmUp, r.WarmUp, r.Method)
					r.doScale(mApp, r.ScaleFactor)
					st.warmUp = 0
				} else {
					log.Infof("%s warming up %s(%d of %d)",
						r.AppID, r.Method, st.warmUp, r.WarmUp)
				}
			} else {
				log.Infof("%s reached max instances %d", r.AppID, r.MaxInstances)
			}
		}
		if sig.Scale.down {
			if mApp.App.Instances > r.MinInstances {
				st.coolDown++
				if st.coolDown >= r.CoolDown {
					log.Infof("%s scale down triggered with %d of %d signals of %s",
						r.AppID, st.coolDown, r.CoolDown, r.Method)
					r.doScale(mApp, -r.ScaleFactor)
					st.coolDown = 0
				} else {
					log.Infof("%s cooling down %s(%d of %d)",
						r.AppID, r.Method, st.coolDown, r.CoolDown)
				}
			} else {
				log.Infof("%s reached min instances %d", r.AppID, r.MinInstances)
			}
		}
	}

}

//EnsureMinMaxInstances scales up or down to get within Min-Max instances
func (r *Scaler) EnsureMinMaxInstances(mApp MarathonApp) bool {
	diff := 0
	if mApp.App.Instances < r.MinInstances {
		diff = r.MinInstances - mApp.App.Instances
		log.Infof("%s will be scaled up by %d to reach minimum instances of %d",
			r.AppID, diff, r.MinInstances)
		r.doScale(mApp, diff)
	} else if mApp.App.Instances > r.MaxInstances {
		diff = r.MaxInstances - mApp.App.Instances
		log.Infof("%s will be scaled down by %d to reach maximum instances of %d",
			r.AppID, diff, r.MaxInstances)
		r.doScale(mApp, diff)
	}
	return diff == 0
}

func (r *Scaler) doScale(mApp MarathonApp, instances int) {
	target := mApp.App.Instances + instances
	if target > r.MaxInstances {
		target = r.MaxInstances
	} else if target < r.MinInstances {
		target = r.MinInstances
	}
	log.Infof("Scaling %s to %d instances", r.AppID, target)
	client.ScaleMarathonApp(r.AppID, target)
}
