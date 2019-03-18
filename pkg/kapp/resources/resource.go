package resources

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

type Resource interface {
	GroupVersionResource() schema.GroupVersionResource
	Kind() string
	APIVersion() string
	APIGroup() string

	Namespace() string
	SetNamespace(name string)
	RemoveNamespace()

	Name() string
	SetName(name string)
	Description() string

	Annotations() map[string]string
	Labels() map[string]string
	OwnerRefs() []metav1.OwnerReference
	Status() map[string]interface{}

	CreatedAt() time.Time
	IsProvisioned() bool
	IsDeleting() bool
	UID() string

	Equal(res Resource) bool
	DeepCopy() Resource
	DeepCopyRaw() map[string]interface{}
	AsYAMLBytes() ([]byte, error)
	AsTypedObj(obj interface{}) error

	markTransient()
	Transient() bool

	unstructured() unstructured.Unstructured     // private
	unstructuredPtr() *unstructured.Unstructured // private
}

type ResourceImpl struct {
	un        unstructured.Unstructured
	gvr       schema.GroupVersionResource
	transient bool
}

var _ Resource = &ResourceImpl{}

func NewResourceUnstructured(un unstructured.Unstructured, gvr schema.GroupVersionResource) *ResourceImpl {
	return &ResourceImpl{un: un, gvr: gvr}
}

func NewResourceFromBytes(data []byte) (*ResourceImpl, error) {
	var content map[string]interface{}

	err := yaml.Unmarshal(data, &content)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return nil, nil
	}

	return &ResourceImpl{un: unstructured.Unstructured{content}}, nil
}

func MustNewResourceFromBytes(data []byte) *ResourceImpl {
	res, err := NewResourceFromBytes(data)
	if err != nil {
		panic(fmt.Sprintf("Invalid resource: %s", err))
	}
	if res == nil {
		panic(fmt.Sprintf("Empty resource: %s", err))
	}
	return res
}

func NewResourcesFromBytes(data []byte) ([]Resource, error) {
	var rs []Resource
	var content map[string]interface{}

	err := yaml.Unmarshal(data, &content)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return nil, nil
	}

	un := unstructured.Unstructured{content}

	if un.IsList() {
		list, err := un.ToList()
		if err != nil {
			return nil, err
		}

		for _, itemUn := range list.Items {
			rs = append(rs, &ResourceImpl{un: itemUn})
		}
	} else {
		rs = append(rs, &ResourceImpl{un: un})
	}

	return rs, nil
}

func (r *ResourceImpl) GroupVersionResource() schema.GroupVersionResource { return r.gvr }

func (r *ResourceImpl) Kind() string       { return r.un.GetKind() }
func (r *ResourceImpl) APIVersion() string { return r.un.GetAPIVersion() }

func (r *ResourceImpl) APIGroup() string {
	pieces := strings.Split(r.APIVersion(), "/")
	if len(pieces) > 2 {
		panic(fmt.Errorf("Expected version to be of format group/version: was %s", r.APIVersion())) // TODO panic
	}
	if len(pieces) == 1 {
		return "" // core API group
	}
	return pieces[0]
}

func (r *ResourceImpl) Namespace() string        { return r.un.GetNamespace() }
func (r *ResourceImpl) SetNamespace(name string) { r.un.SetNamespace(name) }

func (r *ResourceImpl) RemoveNamespace() {
	unstructured.RemoveNestedField(r.un.Object, "metadata", "namespace")
}

func (r *ResourceImpl) Name() string {
	name := r.un.GetName()
	if len(name) > 0 {
		return name
	}
	genName := r.un.GetGenerateName()
	if len(genName) > 0 {
		return genName + "*"
	}
	return ""
}

func (r *ResourceImpl) SetName(name string) { r.un.SetName(name) }

func (r *ResourceImpl) Description() string {
	// TODO proper kind to resource conversion
	result := fmt.Sprintf("%s/%s (%s)", strings.ToLower(r.Kind()), r.Name(), r.APIVersion())

	if len(r.Namespace()) > 0 {
		result += " namespace: " + r.Namespace()
	} else {
		result += " cluster"
	}

	return result
}

func (r *ResourceImpl) CreatedAt() time.Time { return r.un.GetCreationTimestamp().Time }
func (r *ResourceImpl) UID() string          { return string(r.un.GetUID()) }

func (r *ResourceImpl) IsProvisioned() bool {
	// metrics.k8s.io/PodMetrics for example did not have a UID set
	// TODO may be better to rely on selfLink?
	return len(r.un.GetUID()) > 0 || !r.CreatedAt().IsZero()
}

func (r *ResourceImpl) IsDeleting() bool { return r.un.GetDeletionTimestamp() != nil }

func (r *ResourceImpl) markTransient()  { r.transient = true }
func (r *ResourceImpl) Transient() bool { return r.transient }

func (r *ResourceImpl) Annotations() map[string]string     { return r.un.GetAnnotations() }
func (r *ResourceImpl) Labels() map[string]string          { return r.un.GetLabels() }
func (r *ResourceImpl) OwnerRefs() []metav1.OwnerReference { return r.un.GetOwnerReferences() }

func (r *ResourceImpl) Status() map[string]interface{} {
	if r.un.Object != nil {
		if status, ok := r.un.Object["status"]; ok {
			if typedStatus, ok := status.(map[string]interface{}); ok {
				return typedStatus
			}
		}
	}
	return nil
}

func (r *ResourceImpl) Equal(res Resource) bool {
	if typedRes, ok := res.(*ResourceImpl); ok {
		return reflect.DeepEqual(r.un, typedRes.un)
	}
	panic("Resource#Equal only supports ResourceImpl")
}

func (r *ResourceImpl) DeepCopy() Resource {
	return &ResourceImpl{*r.un.DeepCopy(), r.gvr, r.transient}
}

func (r *ResourceImpl) DeepCopyRaw() map[string]interface{} {
	return r.un.DeepCopy().UnstructuredContent()
}

func (r *ResourceImpl) AsYAMLBytes() ([]byte, error) {
	return yaml.Marshal(r.un.Object)
}

func (r *ResourceImpl) AsTypedObj(obj interface{}) error {
	return scheme.Scheme.Convert(r.unstructuredPtr(), obj, nil)
}

func (r *ResourceImpl) unstructured() unstructured.Unstructured     { return r.un }
func (r *ResourceImpl) unstructuredPtr() *unstructured.Unstructured { return &r.un }
