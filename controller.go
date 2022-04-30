package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	clientset   kubernetes.Interface
	cmLister    corelister.ConfigMapLister
	cmhasSynced cache.InformerSynced
	queue       workqueue.RateLimitingInterface
}

func newController(clientset kubernetes.Interface, cmInformer coreinformers.ConfigMapInformer) *controller {
	controller := &controller{
		clientset:   clientset,
		cmLister:    cmInformer.Lister(),
		cmhasSynced: cmInformer.Informer().HasSynced,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "env-sync"),
	}

	cmInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.cmAdded,
			UpdateFunc: controller.cmUpdated,
			DeleteFunc: controller.cmDeleted,
		},
	)
	return c
}

func (c *controller) run(ch <-chan struct{}) {
	fmt.Println("Running controller")
	// Informer maintains a cache that needs to be synced for the first time this brings configMap from default namespace
	if !cache.WaitForCacheSync(ch, c.cmhasSynced) {
		fmt.Printf("\nerror Cache not synced")
	}
	go wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

// worker function operate on the queue and process each item
func (c *controller) worker() {
	fmt.Println("Worker called")
	for c.processItem() {

	}
}

func (c *controller) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Forget(item)
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("\nError getting key from Item %s", err.Error())
		return false
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("\nError split key from Item %s", err.Error())
		return false
	}
	fmt.Printf("\n Sync configMap resource %s from namespace %s", name, ns)
	err = c.syncCM(ns, name)
	if err != nil {
		return false
	}
	return true
}

func (c *controller) syncCM(ns, name string) error {
	ctx := context.Background()

	// Get all deployments that needs to be updated
	deployments, err := c.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		// handle error
		fmt.Printf("\nerror %s, listing deployments\n", err.Error())
	}

	_, err = c.clientset.CoreV1().ConfigMaps(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		// handle error
		if errors.IsNotFound(err) {
			// handle delete event Remove configMap reference from Deployment if present
			for _, deployment := range deployments.Items {
				deleteCmRef := false
				var loc int
				for i, envRef := range deployment.Spec.Template.Spec.Containers[0].EnvFrom {
					if envRef.ConfigMapRef.LocalObjectReference.Name == name {
						deleteCmRef = true
						loc = i
					}
				}
				if !deleteCmRef {
					continue
				}
				fmt.Printf("\nUpdating deployment %s, Removing ConfigMap Ref...%s", deployment.Name, name)
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					// Retrieve the latest version of Deployment before attempting update
					// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
					result, getErr := c.clientset.AppsV1().Deployments(ns).Get(ctx, deployment.Name, metav1.GetOptions{})
					if getErr != nil {
						panic(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
					}
					allEnv := result.Spec.Template.Spec.Containers[0].EnvFrom
					result.Spec.Template.Spec.Containers[0].EnvFrom = append(allEnv[:loc], allEnv[loc+1:]...)
					_, updateErr := c.clientset.AppsV1().Deployments(ns).Update(context.TODO(), result, metav1.UpdateOptions{})
					return updateErr
				})
				if retryErr != nil {
					panic(fmt.Errorf("Update failed: %v", retryErr))
				}
			}
			return nil
		}
		fmt.Printf("\nerror %s, Getting configMap%s\n", name, err.Error())
		return err
	}
	// update all deployment object, Add the configMapRef
	for _, deployment := range deployments.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Retrieve the latest version of Deployment before attempting update
			// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
			result, getErr := c.clientset.AppsV1().Deployments(ns).Get(ctx, deployment.Name, metav1.GetOptions{})
			if getErr != nil {
				panic(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
			}
			addCmRef := true
			for _, envRef := range deployment.Spec.Template.Spec.Containers[0].EnvFrom {
				if envRef.ConfigMapRef.LocalObjectReference.Name == name {
					addCmRef = false
				}
			}
			if !addCmRef {
				return nil
			}
			fmt.Printf("\nUpdating deployment...%s Adding configMap ref %s", deployment.Name, name)

			EnvFrom := v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: name,
					},
				},
			}
			result.Spec.Template.Spec.Containers[0].EnvFrom = append(result.Spec.Template.Spec.Containers[0].EnvFrom, EnvFrom)
			_, updateErr := c.clientset.AppsV1().Deployments(ns).Update(context.TODO(), result, metav1.UpdateOptions{})
			return updateErr
		})
		if retryErr != nil {
			panic(fmt.Errorf("Update failed: %v", retryErr))
		}
	}
	return nil
}

func (c *controller) cmAdded(obj interface{}) {
	fmt.Println("Cm Added")
	c.queue.Add(obj)
}
func (c *controller) cmDeleted(obj interface{}) {
	fmt.Println("Cm Deleted")
	c.queue.Add(obj)
}
func (c *controller) cmUpdated(oldobj, newobj interface{}) {
	fmt.Println("Cm Updated")
	//c.queue.Add(newobj)
}
