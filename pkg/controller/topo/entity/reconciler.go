// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"context"

	"github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	"github.com/onosproject/onos-operator/pkg/controller/util/grpc"
	"github.com/onosproject/onos-operator/pkg/controller/util/k8s"
	"google.golang.org/grpc/status"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.GetLogger("controller", "topo", "entity")

const topoService = "onos-topo"
const topoFinalizer = "topo"

// Add creates a new Entity controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("topo-entity-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Entity
	err = c.Watch(&source.Kind{Type: &v1beta1.Entity{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Kind and requeue the associated entities
	err = c.Watch(&source.Kind{Type: &v1beta1.Kind{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: &kindMapper{mgr.GetClient()},
	})
	if err != nil {
		return err
	}
	// Connect to the topology service
	conn, err := grpc.ConnectAddress("onos-topo:5150")
	if err != nil {
		log.Errorf("grpc dial topo svc error requeueing %#v", err)
		return err
	}

	r.topoClient = topo.NewTopoClient(conn)

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Entity object
type Reconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	config     *rest.Config
	topoClient topo.TopoClient
}

// Reconcile reads that state of the cluster for a Entity object and makes changes based on the state read
// and what is in the Entity.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the Entity instance
	entity := &v1beta1.Entity{}
	err := r.client.Get(context.TODO(), request.NamespacedName, entity)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if entity.DeletionTimestamp == nil {
		log.Infof("Reconciling Entity Create %#v\n%s/%s", request, request.Namespace, request.Name)
		return r.reconcileCreate(entity)
	}
	log.Infof("Reconciling Entity Delete %#v\n%s/%s", request, request.Namespace, request.Name)
	return r.reconcileDelete(entity)
}

func (r *Reconciler) reconcileCreate(entity *v1beta1.Entity) (reconcile.Result, error) {
	// Add the finalizer to the entity if necessary
	if !k8s.HasFinalizer(entity, topoFinalizer) {
		k8s.AddFinalizer(entity, topoFinalizer)
		err := r.client.Update(context.TODO(), entity)
		if err != nil {
			log.Warnf("failed finalizer update requeueing %#v", err)
			return reconcile.Result{}, err
		}
	}

	if r.entityExists(entity) {
		// if err := r.updateEntity(entity); err != nil {
		// 	log.Warnf("failed ENTITY UPDATE requeueing %#v", err)
		// 	return reconcile.Result{}, err
		// }
		return reconcile.Result{}, nil
	} else {
		// If the entity does not exist, create it
		if err := r.createEntity(entity); err != nil {
			log.Warnf("failed ENTITY CREATE requeueing %#v", err.Error())
			return reconcile.Result{}, err
		}
	}
	// Create or update if necessary
	// if err := r.createEntity(entity); err != nil {
	// 	log.Warnf("failed ENTITY CREATE requeueing %#v", err)
	// 	return reconcile.Result{}, err
	// }
	log.Infof("SUCCESS Reconciling ENTITY %s", entity.Name)
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(entity *v1beta1.Entity) (reconcile.Result, error) {
	// If the entity has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(entity, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	// Delete the entity from the topology
	if err := r.deleteEntity(entity); err != nil {
		return reconcile.Result{}, err
	}

	// Once the entity has been deleted, remove the topology finalizer
	controllerutil.RemoveFinalizer(entity, topoFinalizer)
	if err := r.client.Update(context.TODO(), entity); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) entityExists(entity *v1beta1.Entity) bool {
	_, err := r.topoClient.Get(context.TODO(), &topo.GetRequest{ID: topo.ID(entity.Name)})
	return err == nil
}

func (r *Reconciler) updateEntity(entity *v1beta1.Entity) error {
	request := &topo.UpdateRequest{
		Object: &topo.Object{
			ID:   topo.ID(entity.Name),
			Type: topo.Object_ENTITY,
			Obj: &topo.Object_Entity{
				Entity: &topo.Entity{
					KindID: topo.ID(entity.Spec.Kind.Name),
				},
			},
			Attributes: entity.Spec.Attributes,
		},
	}
	_, err := r.topoClient.Update(context.TODO(), request)
	return err
}

func (r *Reconciler) createEntity(entity *v1beta1.Entity) error {
	request := &topo.CreateRequest{
		Object: &topo.Object{
			ID:   topo.ID(entity.Name),
			Type: topo.Object_ENTITY,
			Obj: &topo.Object_Entity{
				Entity: &topo.Entity{
					KindID: topo.ID(entity.Spec.Kind.Name),
				},
			},
			Attributes: entity.Spec.Attributes,
		},
	}
	_, err := r.topoClient.Create(context.TODO(), request)
	return err
}

func (r *Reconciler) deleteEntity(entity *v1beta1.Entity) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(entity.Name),
	}

	_, err := r.topoClient.Delete(context.TODO(), request)
	if err == nil {
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

type kindMapper struct {
	client client.Client
}

func (m *kindMapper) Map(object handler.MapObject) []reconcile.Request {
	kind := object.Object.(*v1beta1.Kind)
	entities := &v1beta1.EntityList{}
	entityFields := map[string]string{
		"spec.kind.name": kind.Name,
	}
	entityOpts := &client.ListOptions{
		Namespace:     kind.Namespace,
		FieldSelector: fields.SelectorFromSet(entityFields),
	}
	err := m.client.List(context.TODO(), entities, entityOpts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(entities.Items))
	for i, entity := range entities.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: entity.Namespace,
				Name:      entity.Name,
			},
		}
	}
	return requests
}
