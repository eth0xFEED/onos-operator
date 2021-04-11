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

package relation

import (
	"context"

	"github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	"github.com/onosproject/onos-operator/pkg/controller/util/grpc"
	"github.com/onosproject/onos-operator/pkg/controller/util/k8s"
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

var log = logging.GetLogger("controller", "topo", "relation")

const topoService = "onos-topo"
const topoFinalizer = "topo"

// Add creates a new Relation controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {

	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("topo-relation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Relation
	err = c.Watch(&source.Kind{Type: &v1beta1.Relation{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Kind and requeue the associated relations
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

// Reconciler reconciles a Relation object
type Reconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	config     *rest.Config
	topoClient topo.TopoClient
}

// Reconcile reads that state of the cluster for a Relation object and makes changes based on the state read
// and what is in the Relation.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Relation %s/%s", request.Namespace, request.Name)
	// Fetch the Relation instance
	relation := &v1beta1.Relation{}
	err := r.client.Get(context.TODO(), request.NamespacedName, relation)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Debugf("NOT REQUEUE Relation not found %s/%s", request.Namespace, request.Name)
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		log.Debugf("DO REQUEUE Relation error %s/%s %+v", request.Namespace, request.Name, err.Error())
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if relation.DeletionTimestamp != nil {
		log.Debugf("RECONCILE deltimestamp nil - DELETE Relation error %s/%s %+v", request.Namespace, request.Name, relation.DeletionTimestamp)
		return r.reconcileDelete(relation)
	}
	if r.relationExists(relation) {
		log.Debugf("RECONCILE EXISTS - UPDATE Relation error %s/%s %+v", request.Namespace, request.Name, relation)
		return r.reconcileUpdate(relation)
	}
	log.Debugf("RECONCILE NOT EXISTS - CREATE Relation error %s/%s %+v", request.Namespace, request.Name, relation)
	return r.reconcileCreate(relation)
}

func (r *Reconciler) reconcileCreate(relation *v1beta1.Relation) (reconcile.Result, error) {
	// Add the finalizer to the relation if necessary
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		controllerutil.AddFinalizer(relation, topoFinalizer)
		err := r.client.Update(context.TODO(), relation)
		if err != nil {
			log.Warnf("RELATION FAILED finalizer update requeueing %#v", err.Error())
			return reconcile.Result{}, err
		}
	}

	resp, err := r.createRelation(relation)
	if err != nil {
		log.Warnf("failed RELATION CREATE requeueing %#v", err)
		return reconcile.Result{}, err
	}

	log.Infof("SUCCESS Reconciling CREATE RELATION %s", relation.Name)
	log.Debugf("SUCCESS Reconciling CREATE RELATION %+v %+v", relation, resp)
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileUpdate(relation *v1beta1.Relation) (reconcile.Result, error) {
	getTopoResp, err := r.topoClient.Get(context.TODO(), &topo.GetRequest{ID: topo.ID(relation.Name)})
	if err != nil {
		return reconcile.Result{}, err
	}

	// resp, err := r.updateRelation(relation, getTopoResp.Object.GetRevision())
	if tRel := getTopoResp.GetObject().GetRelation(); tRel != nil {
		log.Debugf("Reconciling update RELATION %+v %+v", relation, getTopoResp)
		// prune out invalid requests
		if tRel.KindID != topo.ID(relation.Spec.Kind.Name) {
			return reconcile.Result{}, errors.NewInvalid("can not update kind on relation object, must be new instatiation")
		}

		if tRel.SrcEntityID != topo.ID(relation.Spec.Source.Name) ||
			tRel.TgtEntityID != topo.ID(relation.Spec.Target.Name) {
			log.Debugf("Change to spec in  update RELATION %+v %+v", relation, tRel)

			rUpdateVal := getTopoResp.Object.GetRevision() + 1
			log.Debugf("Change to revision in  update RELATION %+v %+v", relation, rUpdateVal)
			_, err = r.updateRelation(relation, rUpdateVal)
			if err != nil {
				log.Warnf("failed RELATION UPDATE requeueing %#v", err.Error())
				return reconcile.Result{}, err
			}
		}
	}

	log.Infof("SUCCESS UPDATE Reconciling RELATION %s", relation.Name)

	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(relation *v1beta1.Relation) (reconcile.Result, error) {
	// If the relation has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	// Delete the relation from the topology
	if err := r.deleteRelation(relation); err != nil {
		return reconcile.Result{}, err
	}

	controllerutil.RemoveFinalizer(relation, topoFinalizer)
	if err := r.client.Update(context.TODO(), relation); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) relationExists(relation *v1beta1.Relation) bool {
	_, err := r.topoClient.Get(context.TODO(), &topo.GetRequest{ID: topo.ID(relation.Name)})
	return err == nil
}

func (r *Reconciler) updateRelation(relation *v1beta1.Relation, revision topo.Revision) (*topo.UpdateResponse, error) {
	request := &topo.UpdateRequest{Object: &topo.Object{
		Revision: revision,
		ID:       topo.ID(relation.Name),
		Type:     topo.Object_RELATION,
		Obj: &topo.Object_Relation{
			Relation: &topo.Relation{
				KindID:      topo.ID(relation.Spec.Kind.Name),
				SrcEntityID: topo.ID(relation.Spec.Source.Name),
				TgtEntityID: topo.ID(relation.Spec.Target.Name),
			},
		},
		Attributes: relation.Spec.Attributes,
	}}
	return r.topoClient.Update(context.TODO(), request)
}
func (r *Reconciler) createRelation(relation *v1beta1.Relation) (*topo.CreateResponse, error) {
	request := &topo.CreateRequest{Object: &topo.Object{
		ID:   topo.ID(relation.Name),
		Type: topo.Object_RELATION,
		Obj: &topo.Object_Relation{
			Relation: &topo.Relation{
				KindID:      topo.ID(relation.Spec.Kind.Name),
				SrcEntityID: topo.ID(relation.Spec.Source.Name),
				TgtEntityID: topo.ID(relation.Spec.Target.Name),
			},
		},
		Attributes: relation.Spec.Attributes,
	}}
	return r.topoClient.Create(context.TODO(), request)
}

func (r *Reconciler) deleteRelation(relation *v1beta1.Relation) error {
	_, err := r.topoClient.Delete(context.TODO(), &topo.DeleteRequest{ID: topo.ID(relation.Name)})
	return err
}

type kindMapper struct {
	client client.Client
}

func (m *kindMapper) Map(object handler.MapObject) []reconcile.Request {
	kind := object.Object.(*v1beta1.Kind)
	relations := &v1beta1.RelationList{}
	relationFields := map[string]string{
		"spec.kind.name": kind.Name,
	}
	relationOpts := &client.ListOptions{
		Namespace:     kind.Namespace,
		FieldSelector: fields.SelectorFromSet(relationFields),
	}
	err := m.client.List(context.TODO(), relations, relationOpts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(relations.Items))
	for i, relation := range relations.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: relation.Namespace,
				Name:      relation.Name,
			},
		}
	}
	return requests
}
