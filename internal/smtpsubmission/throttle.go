package smtpsubmission

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type smtpThrottle struct {
	mutex                    sync.Mutex
	activeSessions           int
	activeSessionsByHost     map[string]int
	authFailuresByKey        map[string][]time.Time
	acceptedMessagesByKey    map[string][]time.Time
	maxConcurrentSessions    int
	maxSessionsPerRemoteHost int
	authFailureLimit         int
	authFailureWindow        time.Duration
	messageLimit             int
	messageWindow            time.Duration
	clockFunc                func() time.Time
}

type smtpAuthReservation struct {
	throttle *smtpThrottle
	key      string
}

type smtpMessageReservation struct {
	throttle   *smtpThrottle
	key        string
	reservedAt time.Time
}

func newSMTPThrottle(configValues Config) *smtpThrottle {
	return &smtpThrottle{
		activeSessionsByHost:     make(map[string]int),
		authFailuresByKey:        make(map[string][]time.Time),
		acceptedMessagesByKey:    make(map[string][]time.Time),
		maxConcurrentSessions:    configValues.MaxConcurrentSessions,
		maxSessionsPerRemoteHost: configValues.MaxSessionsPerRemoteHost,
		authFailureLimit:         configValues.AuthFailureLimit,
		authFailureWindow:        configValues.AuthFailureWindow,
		messageLimit:             configValues.MessageLimit,
		messageWindow:            configValues.MessageWindow,
		clockFunc:                func() time.Time { return time.Now().UTC() },
	}
}

func (throttle *smtpThrottle) acquireSession(remoteHost string) (func(), error) {
	normalizedRemoteHost := normalizeThrottleKeyPart(remoteHost, "unknown")
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	if throttle.activeSessions >= throttle.maxConcurrentSessions {
		return nil, errors.New("smtp_submission.concurrent_sessions_exceeded")
	}
	if throttle.activeSessionsByHost[normalizedRemoteHost] >= throttle.maxSessionsPerRemoteHost {
		return nil, errors.New("smtp_submission.remote_sessions_exceeded")
	}
	throttle.activeSessions++
	throttle.activeSessionsByHost[normalizedRemoteHost]++
	return func() {
		throttle.releaseSession(normalizedRemoteHost)
	}, nil
}

func (throttle *smtpThrottle) releaseSession(remoteHost string) {
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	if throttle.activeSessions > 0 {
		throttle.activeSessions--
	}
	if throttle.activeSessionsByHost[remoteHost] <= 1 {
		delete(throttle.activeSessionsByHost, remoteHost)
		return
	}
	throttle.activeSessionsByHost[remoteHost]--
}

func (throttle *smtpThrottle) reserveAuthAttempt(remoteHost string, username string) (smtpAuthReservation, bool) {
	key := authThrottleKey(remoteHost, username)
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	now := throttle.clockFunc()
	failures := pruneWindow(throttle.authFailuresByKey[key], now.Add(-throttle.authFailureWindow))
	if len(failures) >= throttle.authFailureLimit {
		throttle.authFailuresByKey[key] = failures
		return smtpAuthReservation{}, false
	}
	throttle.authFailuresByKey[key] = append(failures, now)
	return smtpAuthReservation{throttle: throttle, key: key}, true
}

func (reservation smtpAuthReservation) accept() {
	reservation.throttle.clearAuthFailures(reservation.key)
}

func (throttle *smtpThrottle) clearAuthFailures(key string) {
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	delete(throttle.authFailuresByKey, key)
}

func (throttle *smtpThrottle) reserveAcceptedMessage(identityID string) (smtpMessageReservation, bool) {
	key := messageThrottleKey(identityID)
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	now := throttle.clockFunc()
	messages := pruneWindow(throttle.acceptedMessagesByKey[key], now.Add(-throttle.messageWindow))
	if len(messages) >= throttle.messageLimit {
		throttle.acceptedMessagesByKey[key] = messages
		return smtpMessageReservation{}, false
	}
	throttle.acceptedMessagesByKey[key] = append(messages, now)
	return smtpMessageReservation{throttle: throttle, key: key, reservedAt: now}, true
}

func (reservation smtpMessageReservation) cancel() {
	reservation.throttle.cancelMessageReservation(reservation.key, reservation.reservedAt)
}

func (throttle *smtpThrottle) cancelMessageReservation(key string, reservedAt time.Time) {
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	messages := throttle.acceptedMessagesByKey[key]
	for messageIndex, messageTime := range messages {
		if messageTime.Equal(reservedAt) {
			messages = append(messages[:messageIndex], messages[messageIndex+1:]...)
			break
		}
	}
	if len(messages) == 0 {
		delete(throttle.acceptedMessagesByKey, key)
		return
	}
	throttle.acceptedMessagesByKey[key] = messages
}

func (throttle *smtpThrottle) activeSessionCount() int {
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	return throttle.activeSessions
}

func pruneWindow(events []time.Time, cutoff time.Time) []time.Time {
	firstKeptIndex := 0
	for firstKeptIndex < len(events) && !events[firstKeptIndex].After(cutoff) {
		firstKeptIndex++
	}
	if firstKeptIndex == 0 {
		return events
	}
	return append([]time.Time(nil), events[firstKeptIndex:]...)
}

func authThrottleKey(remoteHost string, username string) string {
	normalizedUsername := strings.ToLower(strings.TrimSpace(username))
	if normalizedUsername != "" {
		return "username:" + normalizedUsername
	}
	return "remote:" + normalizeThrottleKeyPart(remoteHost, "unknown")
}

func messageThrottleKey(identityID string) string {
	return normalizeThrottleKeyPart(identityID, "unknown")
}

func normalizeThrottleKeyPart(value string, fallback string) string {
	normalizedValue := strings.TrimSpace(value)
	if normalizedValue == "" {
		return fallback
	}
	return normalizedValue
}
