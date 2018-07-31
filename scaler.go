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
	sig := generateSignal(cpu, mem, r)
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
