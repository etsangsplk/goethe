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

package utilities

import (
	"fmt"
	"github.com/jwells131313/goethe"
	"github.com/jwells131313/goethe/internal"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type goetheData struct {
	tidMux  sync.Mutex
	lastTid int64
}

var globalGoethe goethe.Goethe = newGoethe()

func newGoethe() goethe.Goethe {
	retVal := &goetheData{
		lastTid: 9,
	}

	return retVal
}

// GetGoethe returns the systems goth global
func GetGoethe() goethe.Goethe {
	return globalGoethe
}

func (goth *goetheData) getAndIncrementTid() int64 {
	goth.tidMux.Lock()
	defer goth.tidMux.Unlock()

	goth.lastTid++
	return goth.lastTid
}

func (goth *goetheData) Go(userCall func() error) {
	tid := goth.getAndIncrementTid()

	go invokeStart(tid, userCall)
}

func (goth *goetheData) GetThreadID() int64 {
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

func (goth *goetheData) NewGoetheLock() goethe.Lock {
	return internal.NewReaderWriterLock(goth)
}

// NewBoundedFunctionQueue returns a function queue with the given capacity
func (goth *goetheData) NewBoundedFunctionQueue(capacity uint32) goethe.FunctionQueue {
	return internal.NewFunctionQueue(capacity)
}

// NewErrorQueue returns an error queue with the given capacity.  If errors
// are returned when the ErrorQueue is at capacity the new errors are dropped
func (goth *goetheData) NewErrorQueue(capacity uint32) goethe.ErrorQueue {
	return internal.NewBoundedErrorQueue(capacity)
}

// NewPool is the native implementation of NewPool
func (goth *goetheData) NewPool(name string, minThreads int32, maxThreads int32, idleDecayDuration time.Duration,
	functionQueue goethe.FunctionQueue, errorQueue goethe.ErrorQueue) (goethe.Pool, error) {
	panic("not implemented")
}

// GetPool returns a non-closed pool with the given name.  If not found second
// value returned will be false
func (goth *goetheData) GetPool(string) (goethe.Pool, bool) {
	panic("not implemented")
}

// convertToNibbles returns the nibbles of the string
func convertToNibbles(tid int64) []byte {
	if tid < 0 {
		panic("The tid must not be negative")
	}

	asString := fmt.Sprintf("%x", tid)
	return []byte(asString)
}

func invokeStart(tid int64, userCall func() error) error {
	nibbles := convertToNibbles(tid)

	return internalInvoke(0, nibbles, userCall)
}

func invokeEnd(userCall func() error) error {
	return userCall()
}

func internalInvoke(index int, nibbles []byte, userCall func() error) error {
	if index >= len(nibbles) {
		return invokeEnd(userCall)
	}

	currentFrame := nibbles[index]
	switch currentFrame {
	case byte('0'):
		return xXTidFrame0(index, nibbles, userCall)
	case byte('1'):
		return xXTidFrame1(index, nibbles, userCall)
	case byte('2'):
		return xXTidFrame2(index, nibbles, userCall)
	case byte('3'):
		return xXTidFrame3(index, nibbles, userCall)
	case byte('4'):
		return xXTidFrame4(index, nibbles, userCall)
	case byte('5'):
		return xXTidFrame5(index, nibbles, userCall)
	case byte('6'):
		return xXTidFrame6(index, nibbles, userCall)
	case byte('7'):
		return xXTidFrame7(index, nibbles, userCall)
	case byte('8'):
		return xXTidFrame8(index, nibbles, userCall)
	case byte('9'):
		return xXTidFrame9(index, nibbles, userCall)
	case byte('a'):
		return xXTidFrameA(index, nibbles, userCall)
	case byte('b'):
		return xXTidFrameB(index, nibbles, userCall)
	case byte('c'):
		return xXTidFrameC(index, nibbles, userCall)
	case byte('d'):
		return xXTidFrameD(index, nibbles, userCall)
	case byte('e'):
		return xXTidFrameE(index, nibbles, userCall)
	case byte('f'):
		return xXTidFrameF(index, nibbles, userCall)
	default:
		panic("not yet implemented")

	}

}

func xXTidFrame0(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame1(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame2(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame3(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame4(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame5(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame6(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame7(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame8(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrame9(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameA(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameB(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameC(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameD(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameE(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}

func xXTidFrameF(index int, nibbles []byte, userCall func() error) error {
	return internalInvoke(index+1, nibbles, userCall)
}
