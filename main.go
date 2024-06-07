package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type KeyMetadata struct {
	Key          string    `json:"key"`
	CreationTime time.Time `json:"createdAt"`
	LastAccess   time.Time `json:"lastAccess"`
	IsBlocked    bool      `json:"isBlocked"`
	BlockedAt    time.Time `json:"blockedAt"`
}

type KeyManager struct {
	keys      map[string]KeyMetadata
	available []string
	blocked   map[string]time.Time
	mu        sync.Mutex
	blockMu   sync.Mutex
}

func NewKeyManager() *KeyManager {
	return &KeyManager{
		keys:    make(map[string]KeyMetadata),
		blocked: make(map[string]time.Time),
	}
}

func GenerateRandomKey() string {
	return "key" + strconv.Itoa(rand.Int())
}

func (km *KeyManager) GenerateNewKey() string {
	km.mu.Lock()
	defer km.mu.Unlock()

	newKey := GenerateRandomKey()

	km.keys[newKey] = KeyMetadata{
		Key:          newKey,
		CreationTime: time.Now(),
	}
	fmt.Println(km.keys[newKey])
	km.available = append(km.available, newKey)

	return newKey
}

func (km *KeyManager) RetreiveAvailableKey() (string, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if len(km.available) == 0 {
		return "", errors.New("no keys available")
	}

	index := rand.Intn(len(km.available))
	key := km.available[index]
	km.available = append(km.available[:index], km.available[index+1:]...)

	km.keys[key] = KeyMetadata{
		Key:        key,
		LastAccess: time.Now(),
		IsBlocked:  true,
		BlockedAt:  time.Now(),
	}

	km.blocked[key] = time.Now()
	return key, nil
}

func (km *KeyManager) UnblockKey(key string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if _, exists := km.blocked[key]; exists {
		metadata := km.keys[key]
		metadata.IsBlocked = false
		delete(km.blocked, key)
		km.available = append(km.available, key)
		km.keys[key] = metadata
		return nil
	}

	return errors.New("key not blocked or not exist")
}

func (km *KeyManager) DeleteKey(key string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	delete(km.keys, key)
	delete(km.blocked, key)

	return nil
}

func (km *KeyManager) KeepAlive(key string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if _, exists := km.keys[key]; exists {
		metadata := km.keys[key]
		metadata.LastAccess = time.Now()
		km.keys[key] = metadata
		return nil
	}
	return errors.New("key does not exist")
}

func (km *KeyManager) GetKeyInfo(key string) (KeyMetadata, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	fmt.Println(km.keys[key])
	if metadata, exists := km.keys[key]; exists {
		return metadata, nil
	}
	return KeyMetadata{}, errors.New("key does not exist")
}

func (km *KeyManager) BackgroundTask() {
	for {
		time.Sleep(1 * time.Second)
		now := time.Now()

		km.blockMu.Lock()

		for key, blockedTime := range km.blocked {
			if now.Sub(blockedTime) > 20*time.Second {
				metadata := km.keys[key]
				metadata.IsBlocked = false
				delete(km.blocked, key)
				km.keys[key] = metadata
				km.available = append(km.available, key)
			}
		}
		km.blockMu.Unlock()

		km.mu.Lock()

		for key, metadata := range km.keys {
			if now.Sub(metadata.LastAccess) > 1*time.Minute {
				km.DeleteKey(key)
			}
		}
		km.mu.Unlock()
	}
}

func main() {
	km := NewKeyManager()
	go km.BackgroundTask()

	r := gin.Default()

	r.POST("/keys", func(c *gin.Context) {
		key := km.GenerateNewKey()
		c.JSON(http.StatusCreated, gin.H{"keyId": key})
	})

	r.GET("/keys", func(c *gin.Context) {
		key, err := km.RetreiveAvailableKey()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"keyId": key})
		}
	})

	r.GET("/keys/:id", func(c *gin.Context) {
		key := c.Param("id")
		metadata, err := km.GetKeyInfo(key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, metadata)
		}

	})

	r.DELETE("/keys/:id", func(c *gin.Context) {
		key := c.Param("id")
		err := km.DeleteKey(key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Key is deleted"})
		}
	})

	r.PUT("/keys/:id", func(c *gin.Context) {
		key := c.Param("id")
		err := km.UnblockKey(key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Key is unblocked again"})
		}
	})

	r.PUT("/keepalive/:id", func(c *gin.Context) {
		key := c.Param("id")
		err := km.KeepAlive(key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Key is alive again"})
		}
	})

	r.Run(":8000")
}
