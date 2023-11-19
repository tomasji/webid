package pages

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"sort"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	webidv1alpha1 "github.com/tomasji/webid-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles a Page object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Data   map[types.NamespacedName]PageData
	mu     sync.Mutex
}

type PageData map[string][]byte

const (
	pageFinalizer = "tomasji.github.com/finalizer"
	webServerKey  = "spec.webserver"
)

//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=pages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=pages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=pages/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	debug := log.V(1).Info

	// Get the Page object
	page, err := r.getPage(ctx, req.NamespacedName)
	if page == nil {
		return ctrl.Result{}, err
	}
	debug("Reconcile: got object:", "page", page)

	// Handle deletion
	markedForDeletion := page.GetDeletionTimestamp() != nil

	// Add finalizer if it does not exist
	if !markedForDeletion {
		if err = r.addFinalizer(ctx, page); err != nil {
			return ctrl.Result{}, err
		}
	}

	web, err := r.getWebServer(ctx, types.NamespacedName{Namespace: page.Namespace, Name: page.Spec.WebServer})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get all pages for given webserver, prepare Data
	dataChanged, hash, err := r.prepareData(ctx, page.Namespace, page.Spec.WebServer)
	if err != nil {
		log.Error(err, "Failed to get web page data", "webserver", page.Spec.WebServer)
	}

	// if data's changed, update the web server status and trigger its reconcile
	if dataChanged {
		if err = r.setWebStatus(ctx, web, hash); err != nil {
			return ctrl.Result{}, err
		}
	}

	if markedForDeletion {
		if err = r.removeFinalizer(ctx, page); err != nil {
			return ctrl.Result{}, err
		}
	}

	debug("Reconcile: completed")
	return ctrl.Result{}, nil
}

// getPage retrieves page object, it returns:
// - nil, nil -> stop reconciliation (obj deleted)
// - nil, error -> stop reconciliation (requeue)
// - web, nik -> got it
func (r *Reconciler) getPage(ctx context.Context, namespacedName types.NamespacedName) (page *webidv1alpha1.Page, err error) {
	log := log.FromContext(ctx)

	page = &webidv1alpha1.Page{}
	err = r.Get(ctx, namespacedName, page)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Page resource not found. Ignoring since object must be deleted")
			return nil, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get page")
		return nil, err
	}
	return page, nil
}

// addFinalizer adds a finalizer if needed
func (r *Reconciler) addFinalizer(ctx context.Context, page *webidv1alpha1.Page) error {
	log := log.FromContext(ctx)
	finalizerAdded := controllerutil.AddFinalizer(page, pageFinalizer)
	if !finalizerAdded {
		return nil
	}

	if err := r.Update(ctx, page); err != nil {
		log.Error(err, "Failed to update custom resource to add finalizer")
		return err
	}
	log.V(1).Info("finalizer added for", "page", page.Name)
	return nil
}

// removeFinalizer removes a finalizer if needed
func (r *Reconciler) removeFinalizer(ctx context.Context, page *webidv1alpha1.Page) error {
	log := log.FromContext(ctx)
	finalizerRemoved := controllerutil.RemoveFinalizer(page, pageFinalizer)
	if !finalizerRemoved {
		return nil
	}

	if err := r.Update(ctx, page); err != nil {
		log.Error(err, "Failed to update custom resource to remove finalizer")
		return err
	}
	log.V(1).Info("finalizer removed from", "page", page.Name)
	return nil
}

// getWebServer retrieves webserver object
func (r *Reconciler) getWebServer(ctx context.Context, namespacedName types.NamespacedName) (webserver *webidv1alpha1.WebServer, err error) {
	log := log.FromContext(ctx)

	webserver = &webidv1alpha1.WebServer{}
	if err = r.Get(ctx, namespacedName, webserver); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("WebServer resource for Page not found.", "namespace", namespacedName.Namespace, "name", namespacedName.Name)
		} else {
			log.Error(err, "Failed to get WebServer for Page.", "namespace", namespacedName.Namespace, "name", namespacedName.Name)
		}
		return nil, err
	}
	return webserver, nil
}

// prepareData gets list of Page objects that belong to the given webServer and
// prepares a map of data - if it is different from what is stored in r.Data, update it and return changed=true
func (r *Reconciler) prepareData(ctx context.Context, namespace, webServer string) (changed bool, sha string, err error) {
	log := log.FromContext(ctx)
	debug := log.V(1).Info

	nsName := types.NamespacedName{Namespace: namespace, Name: webServer}

	// Get list of pages for given webserver
	list := &webidv1alpha1.PageList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingFields{webServerKey: webServer},
	}
	err = r.List(ctx, list, opts...)
	if err != nil {
		return false, "", err
	}
	newData := make(map[string][]byte)
	for _, i := range list.Items {
		if i.GetDeletionTimestamp() != nil { // marked for deletion
			debug("Deleting Page", "name", i.Spec.Name)
			continue
		}
		debug("Got Page", "name", i.Spec.Name)
		newData[i.Spec.Name] = []byte(i.Spec.Contents)
	}
	oldData := r.GetData(nsName)

	if r.DataDiffer(oldData, newData) {
		debug("Page data changed, updating")
		r.SetData(nsName, newData)
		hash := makeHash(log, newData)
		return true, hash, nil
	}
	return false, "", nil
}

func makeHash(log logr.Logger, data map[string][]byte) string {
	h := sha1.New()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		io.WriteString(h, k)
		buf := bytes.NewBuffer(data[k])
		if _, err := io.Copy(h, buf); err != nil {
			log.Error(err, "writing sha1")
		}
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// DataDiffer compares 2 maps and returns true if they differ
func (r *Reconciler) DataDiffer(oldData, newData map[string][]byte) bool {
	if oldData == nil && newData == nil {
		return false
	}
	if oldData == nil || newData == nil {
		return true // one is nil, the other one is not
	}
	if len(oldData) != len(newData) {
		return true
	}
	for k, old := range oldData {
		new, exists := newData[k]
		if !exists {
			return true // key does not exist
		}
		if !bytes.Equal(old, new) {
			return true // values differ
		}
	}
	return false
}

// setWebStatus updates status conditions of the webserver object
func (r *Reconciler) setWebStatus(ctx context.Context, web *webidv1alpha1.WebServer, hash string) (err error) {
	const statusReason = "Reconciling"
	log := log.FromContext(ctx)
	debug := log.V(1).Info

	debug("Setting webserver status", "namespace", web.Namespace, "name", web.Name, "hash", hash)
	meta.SetStatusCondition(&web.Status.Conditions, metav1.Condition{
		Type:   "UpToDate",
		Status: metav1.ConditionFalse,
		Reason: "PageChanged", Message: "Reconciling",
	})
	if web.ObjectMeta.Annotations == nil {
		web.ObjectMeta.Annotations = make(map[string]string)
	}
	web.ObjectMeta.Annotations["pages"] = hash
	if err = r.Update(ctx, web); err != nil {
		log.Error(err, "Failed to update WebServer status")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
// Create a new index "spec.webserver" in the cache, so that we can filter by it
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &webidv1alpha1.Page{}, webServerKey,
		func(rawObj client.Object) []string {
			page := rawObj.(*webidv1alpha1.Page)
			return []string{page.Spec.WebServer}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&webidv1alpha1.Page{}).
		WithEventFilter(pageEventFilter()).
		Complete(r)
}

func pageEventFilter() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
	}
}
