// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package v1

import (
	"fmt"
	"time"

	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
)

//GetDeployStatus get deploy status.
//if statefulset or deployment is not nil ,return true
func (a *AppService) GetDeployStatus() bool {
	if a.statefulset != nil || a.deployment != nil {
		return true
	}
	fmt.Println(a.statefulset, a.deployment)
	return false
}

//IsClosed is closed
func (a *AppService) IsClosed() bool {
	if a.statefulset == nil && a.deployment == nil && len(a.pods) == 0 {
		return true
	}
	return false
}

var (
	//RUNNING if stateful or deployment exist and ready pod number is equal to the service Replicas
	RUNNING = "running"
	//CLOSED if app service is not in store
	CLOSED = "closed"
	//STARTING if stateful or deployment exist and ready pod number is less than service Replicas
	STARTING = "starting"
	//STOPPING if stateful and deployment is nil and pod number is not 0
	STOPPING = "stopping"
	//ABNORMAL if stateful or deployment exist and ready pod number is less than service Replicas and all pod status is Error
	ABNORMAL = "abnormal"
	//SOMEABNORMAL if stateful or deployment exist and ready pod number is less than service Replicas and some pod status is Error
	SOMEABNORMAL = "some_abnormal"
	//UNKNOW indeterminacy status
	UNKNOW = "unknow"
	//UPGRADE if store have more than 1 app service
	UPGRADE = "upgrade"
	//BUILDING app service is building
	BUILDING = "building"
	//BUILDEFAILURE app service is build failure
	BUILDEFAILURE = "build_failure"
	//UNDEPLOY init status
	UNDEPLOY = "undeploy"
)

//GetServiceStatus get service status
func (a *AppService) GetServiceStatus() string {
	if a == nil {
		return CLOSED
	}
	if a.statefulset == nil && a.deployment == nil && len(a.pods) == 0 {
		return CLOSED
	}
	if a.statefulset == nil && a.deployment == nil && len(a.pods) > 0 {
		return STOPPING
	}
	if (a.statefulset != nil || a.deployment != nil) && len(a.pods) < a.Replicas {
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
		return RUNNING
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
		return RUNNING
	}

	if a.deployment != nil && (a.deployment.Status.ReadyReplicas < int32(a.Replicas) && a.deployment.Status.ReadyReplicas != 0) {
		if isHaveTerminatedContainer(a.pods) {
			return SOMEABNORMAL
		}
		return STARTING
	}
	if a.deployment != nil && a.deployment.Status.ReadyReplicas == 0 {
		if isHaveTerminatedContainer(a.pods) {
			return ABNORMAL
		}
		return STARTING
	}
	if a.statefulset != nil && (a.statefulset.Status.ReadyReplicas < int32(a.Replicas) && a.deployment.Status.ReadyReplicas != 0) {
		if isHaveTerminatedContainer(a.pods) {
			return SOMEABNORMAL
		}
		return STARTING
	}
	if a.statefulset != nil && a.statefulset.Status.ReadyReplicas == 0 {
		if isHaveTerminatedContainer(a.pods) {
			return ABNORMAL
		}
		return STARTING
	}
	return UNKNOW
}

func isHaveTerminatedContainer(pods []*corev1.Pod) bool {
	for _, pod := range pods {
		for _, con := range pod.Status.ContainerStatuses {
			//have Terminated container
			if con.State.Terminated != nil {
				return true
			}
			if con.LastTerminationState.Terminated != nil {
				return true
			}
		}
	}
	return false
}

//ErrWaitTimeOut wait time out
var ErrWaitTimeOut = fmt.Errorf("Wait time out")

//ErrWaitCancel wait cancel
var ErrWaitCancel = fmt.Errorf("Wait cancel")

//WaitReady wait ready
func (a *AppService) WaitReady(timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a.Ready() {
		return nil
	}
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	select {
	case <-cancel:
		return ErrWaitCancel
	case <-timer.C:
		return ErrWaitTimeOut
	case <-ticker.C:
		if a.Ready() {
			return nil
		}
		a.printLogger(logger)
	}
	return nil
}

//WaitStop wait service stop complate
func (a *AppService) WaitStop(timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a == nil {
		return nil
	}
	if len(a.pods) == 0 && a.statefulset == nil && a.deployment == nil {
		return nil
	}
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	select {
	case <-cancel:
		return ErrWaitCancel
	case <-timer.C:
		return ErrWaitTimeOut
	case <-ticker.C:
		if len(a.pods) == 0 && a.statefulset == nil && a.deployment == nil {
			return nil
		}
		a.printLogger(logger)
	}
	return nil
}
func (a *AppService) printLogger(logger event.Logger) {
	var ready int32
	if a.statefulset != nil {
		ready = a.statefulset.Status.ReadyReplicas
	}
	if a.deployment != nil {
		ready = a.deployment.Status.ReadyReplicas
	}
	logger.Info(fmt.Sprintf("current instance(count:%d ready:%d notready:%d)", len(a.pods), ready, int32(len(a.pods))-ready), map[string]string{"step": "appruntime", "status": "running"})
}

//Ready Whether ready
func (a *AppService) Ready() bool {
	if a.statefulset != nil {
		if a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	if a.deployment != nil {
		if a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	return false
}

//GetReadyReplicas get already ready pod number
func (a *AppService) GetReadyReplicas() int32 {
	if a.statefulset != nil {
		return a.statefulset.Status.ReadyReplicas
	}
	if a.deployment != nil {
		return a.deployment.Status.ReadyReplicas
	}
	return 0
}

//GetRunningVersion get running version
func (a *AppService) GetRunningVersion() string {
	if a.statefulset != nil {
		return a.statefulset.Labels["version"]
	}
	if a.deployment != nil {
		return a.deployment.Labels["version"]
	}
	return ""
}
func (a *AppService) upgradeComlete() bool {
	for _, pod := range a.pods {
		if pod.Labels["version"] != a.DeployVersion {
			return false
		}
	}
	return a.Ready()
}

//WaitUpgradeReady wait upgrade success
func (a *AppService) WaitUpgradeReady(timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a == nil {
		return nil
	}
	if a.upgradeComlete() {
		return nil
	}
	ticker := time.NewTicker(timeout / 10)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	select {
	case <-cancel:
		return ErrWaitCancel
	case <-timer.C:
		return ErrWaitTimeOut
	case <-ticker.C:
		if a.upgradeComlete() {
			return nil
		}
		a.printLogger(logger)
	}
	return nil
}