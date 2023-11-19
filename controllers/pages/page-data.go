package pages

import "k8s.io/apimachinery/pkg/types"

type DataProvider interface {
	GetData(webNsName types.NamespacedName) map[string][]byte
	DataDiffer(oldData, newData map[string][]byte) bool
}

func (r *Reconciler) GetData(webNsName types.NamespacedName) map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Data[webNsName]
}

func (r *Reconciler) SetData(webNsName types.NamespacedName, data map[string][]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Data[webNsName] = data
}
