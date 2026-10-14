/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import "errors"

// InputError reports a request field the caller has to correct; the handlers answer it with 400. Several of them
// may be combined with errors.Join.
type InputError struct {
	Field   string
	Message string
}

func (e *InputError) Error() string {
	return "invalid field " + e.Field + ": " + e.Message
}

// ErrSourceAccessDenied is wrapped by errors stating that the user may not read the source of an export; the
// handlers answer it with 403. A failure to ask the responsible service is never wrapped with it.
var ErrSourceAccessDenied = errors.New("source access denied")
