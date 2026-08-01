package types

import (
	"context"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BotInterface defines the interface that handlers need from the bot
type BotInterface interface {
	SendMessage(chatID int64, text string)
	SendLongMessage(chatID int64, text string)
	SendMarkdown(chatID int64, text string)
	SendKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup)
	SendReplyKeyboard(chatID int64, text string, keyboard tgbotapi.ReplyKeyboardMarkup)
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
	ShowMainMenu(chatID int64, session *UserSession)
	ShowResourceTypes(chatID int64, session *UserSession)
	ShowMonitor(chatID int64, session *UserSession)
	ShowOperations(chatID int64, session *UserSession)
	ShowSettings(chatID int64, session *UserSession)
}

// CommandHandler interface
type CommandHandler interface {
	Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *UserSession) error
}

type UserSession struct {
	UserID         int64
	CurrentNS      string
	CurrentCtx     string
	LastActivity   time.Time
	State          map[string]interface{}
	MenuState      *MenuState
	mu             sync.RWMutex
}

type MenuState struct {
	CurrentView    string
	ResourceType   string
	Namespace      string
	Page           int
	Filter         string
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

// ResourceMap is the shared, exported resource alias map used by all handlers.
var ResourceMap = map[string]SchemaGroupVersionResource{
	"pod":                      {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"pods":                     {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"po":                       {Group: "", Version: "v1", Resource: "pods", Kind: "Pod"},
	"deployment":               {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deployments":              {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"deploy":                   {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
	"service":                  {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"services":                 {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"svc":                      {Group: "", Version: "v1", Resource: "services", Kind: "Service"},
	"replicaset":               {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"replicasets":              {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"rs":                       {Group: "apps", Version: "v1", Resource: "replicasets", Kind: "ReplicaSet"},
	"namespace":                {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"namespaces":               {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"ns":                       {Group: "", Version: "v1", Resource: "namespaces", Kind: "Namespace"},
	"node":                     {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"nodes":                    {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"no":                       {Group: "", Version: "v1", Resource: "nodes", Kind: "Node"},
	"configmap":                {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"configmaps":               {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"cm":                       {Group: "", Version: "v1", Resource: "configmaps", Kind: "ConfigMap"},
	"secret":                   {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"secrets":                  {Group: "", Version: "v1", Resource: "secrets", Kind: "Secret"},
	"pvc":                      {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pvcs":                     {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"persistentvolumeclaim":    {Group: "", Version: "v1", Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim"},
	"pv":                       {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"pvs":                      {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"persistentvolume":         {Group: "", Version: "v1", Resource: "persistentvolumes", Kind: "PersistentVolume"},
	"ingress":                  {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ingresses":                {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"ing":                      {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses", Kind: "Ingress"},
	"event":                    {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"events":                   {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
	"ev":                       {Group: "", Version: "v1", Resource: "events", Kind: "Event"},
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

// Touch updates the LastActivity timestamp (safe for concurrent use).
func (s *UserSession) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// GetNamespace returns the user's current namespace.
func (s *UserSession) GetNamespace() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentNS
}

// SetNamespace sets the user's current namespace.
func (s *UserSession) SetNamespace(ns string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentNS = ns
}

// GetMenuState returns a copy of the current menu state (safe for concurrent use).
func (s *UserSession) GetMenuState() *MenuState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.MenuState == nil {
		return nil
	}
	cp := *s.MenuState
	return &cp
}

// SetMenuState updates the current menu state.
func (s *UserSession) SetMenuState(state *MenuState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MenuState = state
}

// Int64Ptr returns a pointer to the given int64 value.
func Int64Ptr(i int64) *int64 {
	return &i
}

// RateLimiter is a simple per-user sliding-window rate limiter.
type RateLimiter struct {
	requests map[int64][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[int64][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow returns true if the user is within the rate limit.
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

// Cleanup removes stale entries from the rate limiter.
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