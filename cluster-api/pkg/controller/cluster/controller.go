/*
Copyright 2018 The Kubernetes Authors.

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

package cluster

import (
	"log"

	"github.com/craigtracey/kube-deploy/cluster-api/pkg/client/clientset_generated/clientset"
	"github.com/craigtracey/kube-deploy/cluster-api/util"
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/apiserver-builder/pkg/builders"

	"k8s.io/client-go/kubernetes"
	"k8s.io/kube-deploy/cluster-api/pkg/apis/cluster/v1alpha1"
	listers "k8s.io/kube-deploy/cluster-api/pkg/client/listers_generated/cluster/v1alpha1"
	"k8s.io/kube-deploy/cluster-api/pkg/controller/sharedinformers"
)

// +controller:group=cluster,version=v1alpha1,kind=Cluster,resource=clusters
type ClusterControllerImpl struct {
	builders.DefaultControllerFns

	// lister indexes properties about Cluster
	lister listers.ClusterLister

	actuator Actuator

	kubernetesClientSet *kubernetes.Clientset
	clientSet           clientset.Interface
	clusterClient       v1alpha1.ClusterInterface
}

// Init initializes the controller and is called by the generated code
// Register watches for additional resource types here.
func (c *ClusterControllerImpl) Init(arguments sharedinformers.ControllerInitArguments, actuator Actuator) {
	// Use the lister for indexing clusters labels
	c.lister = arguments.GetSharedInformers().Factory.Cluster().V1alpha1().Clusters().Lister()

	clientset, err := clientset.NewForConfig(arguments.GetRestConfig())
	if err != nil {
		glog.Fatalf("error creating cluster client: %v", err)
	}
	c.clientSet = clientset
	c.kubernetesClientSet = arguments.GetSharedInformers().KubernetesClientSet

	c.clusterClient = clientset.ClusterV1alpha1().Clusters(corev1.NamespaceDefault)
	c.actuator = actuator
}

// Reconcile handles enqueued messages
func (c *ClusterControllerImpl) Reconcile(u *v1alpha1.Cluster) error {
	// Implement controller logic here
	log.Printf("Running reconcile Cluster for %s\n", u.Name)

	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		// no-op if finalizer has been removed.
		if !util.Contains(cluster.ObjectMeta.Finalizers, clusterv1.ClusterFinalizer) {
			glog.Infof("reconciling cluster object %v causes a no-op as there is no finalizer.", name)
			return nil
		}

		glog.Infof("reconciling cluster object %v triggers delete.", u.Name)
		if err := c.delete(cluster); err != nil {
			glog.Errorf("Error deleting cluster object %v; %v", u.Name, err)
			return err
		}
		// Remove finalizer on successful deletion.
		glog.Infof("cluster object %v deletion successful, removing finalizer.", u.Name)
		cluster.ObjectMeta.Finalizers = util.Filter(cluster.ObjectMeta.Finalizers, clusterv1.ClusterFinalizer)
		if _, err := c.clusterClient.Update(cluster); err != nil {
			glog.Errorf("Error removing finalizer from cluster object %v; %v", name, err)
			return err
		}
		return nil
	}

	exist, err := c.actuator.Exists(cluster)
	if err != nil {
		glog.Errorf("Error checking existence of cluster instance for cluster object %v; %v", name, err)
		return err
	}
	if exist {
		glog.Infof("reconciling cluster object %v triggers idempotent update.", name)
		return c.update(cluster)
	}
	// Cluster resource created. Cluster does not yet exist.
	glog.Infof("reconciling cluster object %v triggers idempotent create.", cluster.ObjectMeta.Name)
	return c.create(cluster)
}

func (c *ClusterControllerImpl) Get(namespace, name string) (*v1alpha1.Cluster, error) {
	return c.lister.Clusters(namespace).Get(name)
}
