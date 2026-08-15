package types

import (
	"context"
	"sync"
	"time"

	"github.com/ksauraj/telectl/internal/tg"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BotInterface defines the interface that handlers need from the bot.
type BotInterface interface {
	SendMessage(chatID int64, text string)
	SendLongMessage(chatID int64, text string)
	SendMarkdown(chatID int64, text string)
	SendHTML(chatID int64, text string)
	// SendRich sends a Rich Message, falling back to the plain-text rendering
	// if the server rejects it.
	SendRich(chatID int64, markdown, fallback string)
	SendRichKeyboard(chatID int64, markdown, fallback string, keyboard *tg.InlineKeyboardMarkup)
	SendText(chatID int64, text string)
	SendTextFull(chatID int64, text string, parseMode string, keyboard *tg.InlineKeyboardMarkup)
	SendKeyboard(chatID int64, text string, keyboard *tg.InlineKeyboardMarkup)
	SendReplyKeyboard(chatID int64, text string, keyboard *tg.ReplyKeyboardMarkup)
	IsUserAllowed(userID int64) bool
	IsCommandAllowed(command string) bool
	// Typed accessors — return concrete types so handlers can use fields directly
	K8sClient() interface{}
	Config() interface{}
	API() interface{}
	MenuBuilder() interface{}
	Logger() interface{}
	RateLimiter() interface{}
	// Menu view methods
	ShowMainMenu(ctx context.Context, chatID int64, session *UserSession)
	ShowResourceTypes(ctx context.Context, chatID int64, session *UserSession)
	ShowMonitor(ctx context.Context, chatID int64, session *UserSession)
	ShowOperations(ctx context.Context, chatID int64, session *UserSession)
	ShowSettings(ctx context.Context, chatID int64, session *UserSession)
	// Build info
	BuildVersion() string
	BuildCommit() string
	BuildDate() string
}

// CommandHandler interface.
type CommandHandler interface {
	Handle(ctx context.Context, msg *tg.Message, args []string, session *UserSession) error
}

type UserSession struct {
	UserID       int64
	CurrentNS    string
	CurrentCtx   string
	LastActivity time.Time
	State        map[string]interface{}
	MenuState    *MenuState
	mu           sync.RWMutex
}

type MenuState struct {
	CurrentView  string
	ResourceType string
	Namespace    string
	Page         int
	Filter       string
}

// SchemaGroupVersionResource is the typed alias for schema.GroupVersionResource
// with a Kind field for resource lookups.
type SchemaGroupVersionResource struct {
	Group    string
	Version  string
	Resource string
	Kind     string
}

// GVR returns the schema.GroupVersionResource for use with the dynamic client.
func (s SchemaGroupVersionResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: s.Group, Version: s.Version, Resource: s.Resource}
}

// clusterScopedResources are the plural resource names that exist outside any
// namespace. Passing a namespace when querying these makes the API server look
// for a namespaced variant, which does not exist, and it answers "the server
// could not find the requested resource".
var clusterScopedResources = map[string]bool{
	"namespaces":        true,
	"nodes":             true,
	"persistentvolumes": true,
}

// IsClusterScoped reports whether a resource alias refers to a cluster-scoped
// kind. Accepts any alias in ResourceMap (e.g. "ns", "no", "pv").
func IsClusterScoped(alias string) bool {
	gvr, ok := ResourceMap[alias]
	if !ok {
		return false
	}
	return clusterScopedResources[gvr.Resource]
}

// ResourceMap is the shared, exported resource alias map used by all handlers.
var ResourceMap = map[string]SchemaGroupVersionResource{
	"pod":                   {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"pods":                  {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"po":                    {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"deployment":            {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deployments":           {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deploy":                {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"service":               {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"services":              {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"svc":                   {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"replicaset":            {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"replicasets":           {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"rs":                    {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"namespace":             {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"namespaces":            {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"ns":                    {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"node":                  {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"nodes":                 {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"no":                    {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"configmap":             {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"configmaps":            {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"cm":                    {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"secret":                {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"secrets":               {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"pvc":                   {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pvcs":                  {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"persistentvolumeclaim": {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pv":                    {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"pvs":                   {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"persistentvolume":      {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"ingress":               {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ingresses":             {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ing":                   {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"event":                 {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"events":                {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"ev":                    {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
}

func (s *UserSession) IsInExecMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.State["exec_mode"]
	return ok
}

func (s *UserSession) SetExecMode(pod, namespace, container string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State["exec_mode"] = true
	s.State["exec_pod"] = pod
	s.State["exec_namespace"] = namespace
	s.State["exec_container"] = container
}

func (s *UserSession) ClearExecMode() {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.State, "exec_mode")
	delete(s.State, "exec_pod")
	delete(s.State, "exec_namespace")
	delete(s.State, "exec_container")
}

func (s *UserSession) GetExecInfo() (pod, namespace, container string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.State["exec_pod"].(string); ok {
		pod = v
	}
	if v, ok := s.State["exec_namespace"].(string); ok {
		namespace = v
	}
	if v, ok := s.State["exec_container"].(string); ok {
		container = v
	}
	return
}

func (s *UserSession) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

func (s *UserSession) GetNamespace() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentNS
}

func (s *UserSession) SetNamespace(ns string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentNS = ns
}

func (s *UserSession) GetMenuState() *MenuState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.MenuState == nil {
		return nil
	}
	cp := *s.MenuState
	return &cp
}

func (s *UserSession) SetMenuState(state *MenuState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MenuState = state
}

// GetState retrieves a value from the session's state map.
func (s *UserSession) GetState(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.State == nil {
		return nil, false
	}
	v, ok := s.State[key]
	return v, ok
}

// SetState stores a value in the session's state map.
func (s *UserSession) SetState(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State == nil {
		s.State = make(map[string]interface{})
	}
	s.State[key] = value
}

// DeleteState removes a value from the session's state map.
func (s *UserSession) DeleteState(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != nil {
		delete(s.State, key)
	}
}

func Int64Ptr(i int64) *int64 {
	return &i
}

type RateLimiter struct {
	requests map[int64][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[int64][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	requests := rl.requests[userID]
	valid := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		return false
	}

	rl.requests[userID] = append(valid, now)
	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for userID, requests := range rl.requests {
		valid := make([]time.Time, 0, len(requests))
		for _, t := range requests {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, userID)
		} else {
			rl.requests[userID] = valid
		}
	}
}
