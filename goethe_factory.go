/*
 * DO NOT ALTER OR REMOVE COPYRIGHT NOTICES OR THIS HEADER.
 *
 * Copyright (c) 2018 Oracle and/or its affiliates. All rights reserved.
 *
 * The contents of this file are subject to the terms of either the GNU
 * General Public License Version 2 only ("GPL") or the Common Development
 * and Distribution License("CDDL") (collectively, the "License").  You
 * may not use this file except in compliance with the License.  You can
 * obtain a copy of the License at
 * https://glassfish.dev.java.net/public/CDDL+GPL_1_1.html
 * or packager/legal/LICENSE.txt.  See the License for the specific
 * language governing permissions and limitations under the License.
 *
 * When distributing the software, include this License Header Notice in each
 * file and include the License file at packager/legal/LICENSE.txt.
 *
 * GPL Classpath Exception:
 * Oracle designates this particular file as subject to the "Classpath"
 * exception as provided by Oracle in the GPL Version 2 section of the License
 * file that accompanied this code.
 *
 * Modifications:
 * If applicable, add the following below the License Header, with the fields
 * enclosed by brackets [] replaced by your own identifying information:
 * "Portions Copyright [year] [name of copyright owner]"
 *
 * Contributor(s):
 * If you wish your version of this file to be governed by only the CDDL or
 * only the GPL Version 2, indicate your decision by adding "[Contributor]
 * elects to include this software in this distribution under the [CDDL or GPL
 * Version 2] license."  If you don't indicate a single choice of license, a
 * recipient has the option to distribute your version of this file under
 * either the CDDL, the GPL Version 2 or to extend the choice of license to
 * its licensees as provided above.  However, if you add GPL Version 2 code
 * and therefore, elected the GPL Version 2 license, then the option applies
 * only if the new code is made subject to such option by the copyright
 * holder.
 */

package goethe

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type poolData struct {
	poolMux sync.Mutex
	poolMap map[string]Pool
}

type timersData struct {
	timerMux sync.Mutex
	timer    timerImpl
}

type threadLocalsData struct {
	localsMux    sync.Mutex
	threadLocals map[string]*threadLocalOperators
}

// StandardThreadUtilities provides methods for using the goethe threading
// system, including timers, pools, recursive locks,
// and thread pools.  It implements the ThreadUtilities interface
// which is what the GG and GetGoethe methods return
type StandardThreadUtilities struct {
	tidMux  sync.Mutex
	lastTid int64

	pools  *poolData
	timers *timersData
	locals *threadLocalsData
}

type threadLocalOperators struct {
	initializer func(ThreadLocal) error
	destroyer   func(ThreadLocal) error
	lock        Lock
	actuals     map[int64]ThreadLocal
}

var (
	errorType    = reflect.TypeOf(errors.New("")).String()
	globalGoethe = newGoethe()
)

const (
	timerTid = 9
)

func newGoethe() *StandardThreadUtilities {
	pools := &poolData{
		poolMap: make(map[string]Pool),
	}

	timers := &timersData{}

	locals := &threadLocalsData{
		threadLocals: make(map[string]*threadLocalOperators),
	}

	retVal := &StandardThreadUtilities{
		lastTid: 9,
		pools:   pools,
		timers:  timers,
		locals:  locals,
	}

	return retVal
}

// GetGoethe returns the systems goethe global
func GetGoethe() ThreadUtilities {
	return GG()
}

// GG returns the system goethe global implementation, and also means "Good Game"
func GG() ThreadUtilities {
	return globalGoethe
}

func (goth *StandardThreadUtilities) getAndIncrementTid() int64 {
	goth.tidMux.Lock()
	defer goth.tidMux.Unlock()

	goth.lastTid++
	return goth.lastTid
}

// Go takes as a first argument any function and
// all the remaining fields are the arguments to that function
// it is up to the caller to maintain type safety
// If this method detects any discrepancy between the
// function passed in and the number and/or type or arguments
// an error is returned.  The thread id is also returned
func (goth *StandardThreadUtilities) Go(userCall interface{}, args ...interface{}) (int64, error) {
	tid := goth.getAndIncrementTid()

	argArray := make([]interface{}, len(args))
	for index, arg := range args {
		argArray[index] = arg
	}

	arguments, err := getValues(userCall, argArray)
	if err != nil {
		return -1, err
	}

	go invokeStart(tid, userCall, arguments)

	return tid, nil
}

// GetThreadID Gets the current threadID.  Returns -1
// if this is not a goethe thread.  Thread ids start at 10
// as thread ids 0 through 9 are reserved for future use
func (goth *StandardThreadUtilities) GetThreadID() int64 {
	stackAsBytes := debug.Stack()
	stackAsString := string(stackAsBytes)

	tokenized := strings.Split(stackAsString, "xXTidFrame")

	var tidHexString string
	first := true
	gotOne := false
	for _, tok := range tokenized {
		if first {
			first = false
		} else {
			gotOne = true
			tidHexString = string(tok[0]) + tidHexString
		}
	}

	if !gotOne {
		return -1
	}

	var result int

	fmt.Sscanf(tidHexString, "%X", &result)

	return int64(result)
}

// NewGoetheLock Creates a new goethe lock
func (goth *StandardThreadUtilities) NewGoetheLock() Lock {
	return newReaderWriterLock(goth)
}

// NewPool creates a new thread pool with the given parameters.  The name is the
// name of this pool and may not be empty.  It is an error to try to create more than
// one open pool with the same name at the same time.
// minThreads is the minimum number of  threads that this pool will maintain while it is open.
// minThreads may be zero. maxThreads is the maximum number of threads this pool will ever
// allocate simultaneously.  New threads will be allocated if all of the threads in the
// pool are busy and the FunctionQueue is not empty (and the total number of threads is less
// than maxThreads) maxThreads must be greater than or equal to minThreads.  Having min and max
// threads both be zero is an error.  Having min and max threads be the same value implies
// a fixed thread size pool.  The idleDecayDuration is how long the system will wait
// while the number of threads is greater than minThreads before removing ending the
// thread.  functionQueue may not be nil and is how functions are enqueued onto the
// thread pool.  errorQueue may be nil but if not nil any error returned by the function
// will be enqueued onto the errorQueue.  It is recommended that the implementation of
// ErrorQueue have some sort of upper bound.  If a pool with the given name already
// exists the old pool will be returned along with an ErrPoolAlreadyExists error
func (goth *StandardThreadUtilities) NewPool(name string, minThreads int32, maxThreads int32, idleDecayDuration time.Duration,
	functionQueue FunctionQueue, errorQueue ErrorQueue) (Pool, error) {
	goth.pools.poolMux.Lock()
	defer goth.pools.poolMux.Unlock()

	foundPool, found := goth.pools.poolMap[name]
	if found {
		return foundPool, ErrPoolAlreadyExists
	}

	retVal, err := newThreadPool(goth, name, minThreads, maxThreads, idleDecayDuration, functionQueue,
		errorQueue)
	if err != nil {
		return nil, err
	}

	goth.pools.poolMap[name] = retVal

	return retVal, nil
}

// GetPool returns a non-closed pool with the given name.  If not found second
// value returned will be false
func (goth *StandardThreadUtilities) GetPool(name string) (Pool, bool) {
	goth.pools.poolMux.Lock()
	goth.pools.poolMux.Unlock()

	retVal, found := goth.pools.poolMap[name]

	return retVal, found
}

// EstablishThreadLocal tells the system of the named thread local storage
// initialize method and destroy method.  This method can be called on any
// thread, including non-goethe threads.  Both the initializer and
// destroyer methods may be nil.  Any errors thrown by these function
// will be put on the error queue
func (goth *StandardThreadUtilities) EstablishThreadLocal(name string, initializer func(ThreadLocal) error,
	destroyer func(ThreadLocal) error) error {
	goth.locals.localsMux.Lock()
	goth.locals.localsMux.Unlock()

	_, found := goth.locals.threadLocals[name]
	if found {
		return fmt.Errorf("There is already an established thread local for %s", name)
	}

	operation := &threadLocalOperators{
		initializer: initializer,
		destroyer:   destroyer,
		lock:        goth.NewGoetheLock(),
		actuals:     make(map[int64]ThreadLocal),
	}

	goth.locals.threadLocals[name] = operation

	return nil
}

// GetThreadLocal returns the instance of the storage associated with
// the current goethe thread.  May only be called on goethe threads and
// will return ErrNotGoetheThread if called from a non-goethe thread.
// If EstablishThreadLocal with the given name has not been called prior to
// this function call then a ThreadLocal with no initializer/destroyer
// methods will be used
func (goth *StandardThreadUtilities) GetThreadLocal(name string) (ThreadLocal, error) {
	tid := goth.GetThreadID()
	if tid < int64(0) {
		return nil, ErrNotGoetheThread
	}

	operators, found := goth.getOperatorsByName(name)
	if !found {
		operators = &threadLocalOperators{
			lock:    goth.NewGoetheLock(),
			actuals: make(map[int64]ThreadLocal),
		}

		goth.locals.localsMux.Lock()
		goth.locals.threadLocals[name] = operators
		goth.locals.localsMux.Unlock()
	}

	operators.lock.WriteLock()
	defer operators.lock.WriteUnlock()

	actual, found := operators.actuals[tid]
	if !found {
		actual = newThreadLocal(name, goth, tid)

		if operators.initializer != nil {
			operators.initializer(actual)
		}

		operators.actuals[tid] = actual
	}

	return actual, nil
}

func (goth *StandardThreadUtilities) startTimer() {
	goth.timers.timerMux.Lock()
	defer goth.timers.timerMux.Unlock()

	if goth.timers.timer != nil {
		return
	}

	goth.timers.timer = newTimer()

	// Add system job
	values := make([]reflect.Value, 0)
	goth.timers.timer.addJob(0, 24*time.Hour, nil,
		func() {
		}, values, false)

	goth.Go(goth.timers.timer.run)

	goth.EstablishThreadLocal(TimerThreadLocal, nil, nil)
}

// ScheduleAtFixedRate schedules the given method with the given args at
// a fixed rate.  The duration of the method does not affect when the
// next method will be run.  The first run will happen only after initialDelay
// and will then be scheduled at multiples of the period.  An optional
// error queue can be given to collect all errors thrown from the method.
// It is the responsibility of the caller to drain the error queue
func (goth *StandardThreadUtilities) ScheduleAtFixedRate(initialDelay time.Duration, period time.Duration,
	errorQueue ErrorQueue, method interface{}, args ...interface{}) (Timer, error) {
	goth.startTimer()

	if period < 1 {
		return nil, fmt.Errorf("Invalid rate of %d given to ScheduledAtFixedRate", period)
	}

	argArray := make([]interface{}, len(args))
	for index, arg := range args {
		argArray[index] = arg
	}

	arguments, err := getValues(method, argArray)
	if err != nil {
		return nil, err
	}

	return goth.timers.timer.addJob(initialDelay, period, errorQueue, method, arguments, true)
}

// ScheduleWithFixedDelay schedules the given method with the given args
// and will schedule the next run after the method returns and the delay has passed.
// The first run will happen only after initialDelay
// An optional error queue can be given to collect all errors thrown from the method.
// It is the responsibility of the caller to drain the error queue
func (goth *StandardThreadUtilities) ScheduleWithFixedDelay(initialDelay time.Duration, delay time.Duration,
	errorQueue ErrorQueue, method interface{}, args ...interface{}) (Timer, error) {
	goth.startTimer()

	if delay < 0 {
		return nil, fmt.Errorf("Invalid delay of %d given to ScheduleWithFixedDelay", delay)
	}

	argArray := make([]interface{}, len(args))
	for index, arg := range args {
		argArray[index] = arg
	}

	arguments, err := getValues(method, argArray)
	if err != nil {
		return nil, err
	}

	return goth.timers.timer.addJob(initialDelay, delay, errorQueue, method, arguments, false)
}

func (goth *StandardThreadUtilities) getOperatorsByName(name string) (*threadLocalOperators, bool) {
	goth.locals.localsMux.Lock()
	goth.locals.localsMux.Unlock()

	retVal, found := goth.locals.threadLocals[name]

	return retVal, found
}

func removeThreadLocal(operators *threadLocalOperators, tid int64) {
	operators.lock.WriteLock()
	defer operators.lock.WriteUnlock()

	actual, found := operators.actuals[tid]
	if !found {
		return
	}

	if operators.destroyer != nil {
		operators.destroyer(actual)
	}

	delete(operators.actuals, tid)
}

func (goth *StandardThreadUtilities) removeAllActuals(tid int64) {
	goth.locals.localsMux.Lock()
	goth.locals.localsMux.Unlock()

	for _, operators := range goth.locals.threadLocals {
		removeThreadLocal(operators, tid)
	}
}

func (goth *StandardThreadUtilities) removePool(name string) {
	goth.pools.poolMux.Lock()
	goth.pools.poolMux.Unlock()

	delete(goth.pools.poolMap, name)
}

// convertToNibbles returns the nibbles of the string
func convertToNibbles(tid int64) []byte {
	if tid < 0 {
		panic("The tid must not be negative")
	}

	asString := fmt.Sprintf("%x", tid)
	return []byte(asString)
}

func invokeStart(tid int64, userCall interface{}, args []reflect.Value) error {
	nibbles := convertToNibbles(tid)

	return internalInvoke(tid, 0, nibbles, userCall, args)
}

func invokeEnd(tid int64, userCall interface{}, args []reflect.Value) error {
	defer globalGoethe.removeAllActuals(tid)

	invoke(userCall, args, nil)

	return nil
}

func internalInvoke(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	if index >= len(nibbles) {
		return invokeEnd(tid, userCall, args)
	}

	currentFrame := nibbles[index]
	switch currentFrame {
	case byte('0'):
		return xXTidFrame0(tid, index, nibbles, userCall, args)
	case byte('1'):
		return xXTidFrame1(tid, index, nibbles, userCall, args)
	case byte('2'):
		return xXTidFrame2(tid, index, nibbles, userCall, args)
	case byte('3'):
		return xXTidFrame3(tid, index, nibbles, userCall, args)
	case byte('4'):
		return xXTidFrame4(tid, index, nibbles, userCall, args)
	case byte('5'):
		return xXTidFrame5(tid, index, nibbles, userCall, args)
	case byte('6'):
		return xXTidFrame6(tid, index, nibbles, userCall, args)
	case byte('7'):
		return xXTidFrame7(tid, index, nibbles, userCall, args)
	case byte('8'):
		return xXTidFrame8(tid, index, nibbles, userCall, args)
	case byte('9'):
		return xXTidFrame9(tid, index, nibbles, userCall, args)
	case byte('a'):
		return xXTidFrameA(tid, index, nibbles, userCall, args)
	case byte('b'):
		return xXTidFrameB(tid, index, nibbles, userCall, args)
	case byte('c'):
		return xXTidFrameC(tid, index, nibbles, userCall, args)
	case byte('d'):
		return xXTidFrameD(tid, index, nibbles, userCall, args)
	case byte('e'):
		return xXTidFrameE(tid, index, nibbles, userCall, args)
	case byte('f'):
		return xXTidFrameF(tid, index, nibbles, userCall, args)
	default:
		panic("unknown type")

	}

}

func xXTidFrame0(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame1(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame2(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame3(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame4(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame5(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame6(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame7(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame8(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrame9(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameA(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameB(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameC(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameD(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameE(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}

func xXTidFrameF(tid int64, index int, nibbles []byte, userCall interface{}, args []reflect.Value) error {
	return internalInvoke(tid, index+1, nibbles, userCall, args)
}
