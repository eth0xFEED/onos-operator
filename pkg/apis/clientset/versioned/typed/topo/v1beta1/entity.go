/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"time"

	scheme "github.com/onosproject/onos-operator/pkg/apis/clientset/versioned/scheme"
	v1beta1 "github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// EntitiesGetter has a method to return a EntityInterface.
// A group's client should implement this interface.
type EntitiesGetter interface {
	Entities(namespace string) EntityInterface
}

// EntityInterface has methods to work with Entity resources.
type EntityInterface interface {
	Create(*v1beta1.Entity) (*v1beta1.Entity, error)
	Update(*v1beta1.Entity) (*v1beta1.Entity, error)
	UpdateStatus(*v1beta1.Entity) (*v1beta1.Entity, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.Entity, error)
	List(opts v1.ListOptions) (*v1beta1.EntityList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.Entity, err error)
	EntityExpansion
}

// entities implements EntityInterface
type entities struct {
	client rest.Interface
	ns     string
}

// newEntities returns a Entities
func newEntities(c *TopoV1beta1Client, namespace string) *entities {
	return &entities{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the entity, and returns the corresponding entity object, and an error if there is any.
func (c *entities) Get(name string, options v1.GetOptions) (result *v1beta1.Entity, err error) {
	result = &v1beta1.Entity{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("entities").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Entities that match those selectors.
func (c *entities) List(opts v1.ListOptions) (result *v1beta1.EntityList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.EntityList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("entities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested entities.
func (c *entities) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("entities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a entity and creates it.  Returns the server's representation of the entity, and an error, if there is any.
func (c *entities) Create(entity *v1beta1.Entity) (result *v1beta1.Entity, err error) {
	result = &v1beta1.Entity{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("entities").
		Body(entity).
		Do().
		Into(result)
	return
}

// Update takes the representation of a entity and updates it. Returns the server's representation of the entity, and an error, if there is any.
func (c *entities) Update(entity *v1beta1.Entity) (result *v1beta1.Entity, err error) {
	result = &v1beta1.Entity{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("entities").
		Name(entity.Name).
		Body(entity).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *entities) UpdateStatus(entity *v1beta1.Entity) (result *v1beta1.Entity, err error) {
	result = &v1beta1.Entity{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("entities").
		Name(entity.Name).
		SubResource("status").
		Body(entity).
		Do().
		Into(result)
	return
}

// Delete takes name of the entity and deletes it. Returns an error if one occurs.
func (c *entities) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("entities").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *entities) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("entities").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched entity.
func (c *entities) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.Entity, err error) {
	result = &v1beta1.Entity{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("entities").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
