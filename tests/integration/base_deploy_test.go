/*
Copyright 2016 The Rook Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/rook/rook/pkg/model"
	"github.com/rook/rook/tests/framework/clients"
	"github.com/rook/rook/tests/framework/contracts"
	"github.com/rook/rook/tests/framework/installer"
	"github.com/rook/rook/tests/framework/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

var (
	logger = capnslog.NewPackageLogger("github.com/rook/rook", "integrationTest")

	defaultNamespace = "default"
)

//Test to make sure all rook components are installed and Running
func checkIfRookClusterIsInstalled(s suite.Suite, k8sh *utils.K8sHelper, opNamespace, clusterNamespace string, mons int) {
	logger.Infof("Make sure all Pods in Rook Cluster %s are running", clusterNamespace)
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-operator", opNamespace, 1, "Running"),
		"Make sure there is 1 rook-operator present in Running state")
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-agent", opNamespace, 1, "Running"),
		"Make sure there is 1 rook-agent present in Running state")
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-api", clusterNamespace, 1, "Running"),
		"Make sure there is 1 rook-api present in Running state")
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-ceph-mgr", clusterNamespace, 1, "Running"),
		"Make sure there is 1 rook-ceph-mgr present in Running state")
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-ceph-osd", clusterNamespace, 1, "Running"),
		"Make sure there is at lest 1 rook-ceph-osd present in Running state")
	assert.True(s.T(), k8sh.CheckPodCountAndState("rook-ceph-mon", clusterNamespace, mons, "Running"),
		fmt.Sprintf("Make sure there are %d rook-ceph-mon present in Running state", mons))
}

func checkIfRookClusterIsHealthy(s suite.Suite, testClient *clients.TestClient, clusterNamespace string) {
	logger.Infof("Testing cluster %s health", clusterNamespace)
	var err error
	var status model.StatusDetails

	retryCount := 0
	for retryCount < utils.RetryLoop {
		status, err = clients.IsClusterHealthy(testClient)
		if err == nil {
			logger.Infof("cluster %s is healthy. final status: %+v", clusterNamespace, status)
			return
		}

		retryCount++
		logger.Infof("waiting for cluster %s to become healthy. err: %+v", clusterNamespace, err)
		<-time.After(time.Duration(utils.RetryInterval) * time.Second)
	}

	require.Nil(s.T(), err)
}

func HandlePanics(r interface{}, op contracts.Setup, t func() *testing.T) {
	if r != nil {
		logger.Infof("unexpected panic occurred during test %s, --> %v", t().Name(), r)
		t().Fail()
		op.TearDown()
		t().FailNow()
	}

}

//GetTestClient sets up SetTestClient for rook
func GetTestClient(kh *utils.K8sHelper, namespace string, op contracts.Setup, t func() *testing.T) *clients.TestClient {
	helper, err := clients.CreateTestClient(kh, namespace)
	if err != nil {
		logger.Errorf("Cannot create rook test client, er -> %v", err)
		t().Fail()
		op.TearDown()
		t().FailNow()
	}
	return helper
}

//BaseTestOperations struct for handling panic and test suite tear down
type BaseTestOperations struct {
	installer       *installer.InstallHelper
	kh              *utils.K8sHelper
	helper          *clients.TestClient
	T               func() *testing.T
	namespace       string
	storeType       string
	dataDirHostPath string
	helmInstalled   bool
	useDevices      bool
	mons            int
}

//NewBaseTestOperations creates new instance of BaseTestOperations struct
func NewBaseTestOperations(t func() *testing.T, namespace, storeType, dataDirHostPath string, helmInstalled, useDevices bool, mons int) (BaseTestOperations, *utils.K8sHelper) {
	kh, err := utils.CreateK8sHelper(t)
	require.NoError(t(), err)

	i := installer.NewK8sRookhelper(kh.Clientset, t)

	op := BaseTestOperations{i, kh, nil, t, namespace, storeType, dataDirHostPath, helmInstalled, useDevices, mons}
	op.SetUp()
	return op, kh
}

//SetUpRook is a wrapper for setting up rook
func (op BaseTestOperations) SetUp() {
	isRookInstalled, err := op.installer.InstallRookOnK8sWithHostPathAndDevices(op.namespace, op.storeType, op.dataDirHostPath, op.helmInstalled, op.useDevices, op.mons)
	assert.NoError(op.T(), err)
	if !isRookInstalled {
		logger.Errorf("Rook Was not installed successfully")
		op.T().Fail()
		op.TearDown()
		op.T().FailNow()
	}
}

//TearDownRook is a wrapper for tearDown after Sutie
func (op BaseTestOperations) TearDown() {
	if op.installer.T().Failed() {
		op.installer.GatherAllRookLogs(op.namespace, op.installer.T().Name())
	}
	op.installer.UninstallRookFromK8s(op.namespace, op.helmInstalled)
}
