/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package requestengine

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The specification asks for a language-appropriate structure when the errors
// array of a cloud answer holds more than one entry, so each entry is its own
// error and the status code and headers stay with it.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-request-engine.md#http-requests
func TestEveryEntryOfACloudErrorsArrayIsItsOwnError(t *testing.T) {
	want := []string{"first thing wrong", "second thing wrong"}
	cs := &cloudServer{
		jsonStatus: http.StatusBadRequest,
		jsonBody:   `{"errors":["` + want[0] + `","` + want[1] + `"]}`,
	}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	err = processAndRead(p.CreateFlowData())
	if err == nil {
		t.Fatal("an answer holding two errors entries gave no error")
	}

	var all *CloudRequestErrors
	if !errors.As(err, &all) {
		t.Fatalf("want a CloudRequestErrors, got %T: %v", err, err)
	}
	if len(all.Errors) != len(want) {
		t.Fatalf("want %d entries, got %d: %v", len(want), len(all.Errors), all.Errors)
	}
	for i, entry := range all.Errors {
		if entry.Message != want[i] {
			t.Errorf("entry %d is %q, want %q", i, entry.Message, want[i])
		}
		if entry.StatusCode != http.StatusBadRequest {
			t.Errorf("entry %d carries status %d, want %d",
				i, entry.StatusCode, http.StatusBadRequest)
		}
		if entry.Headers == nil {
			t.Errorf("entry %d carries no response headers", i)
		}
	}

	// isDefinitive and signalsServiceTrouble read the status code through
	// errors.As, so one entry has to be reachable that way.
	var one *CloudRequestError
	if !errors.As(err, &one) {
		t.Fatalf("errors.As reaches no CloudRequestError in %v", err)
	}
	if one.StatusCode != http.StatusBadRequest {
		t.Errorf("the CloudRequestError errors.As reached carries status %d, want %d",
			one.StatusCode, http.StatusBadRequest)
	}
	if !errors.Is(err, all.Errors[len(all.Errors)-1]) {
		t.Error("errors.Is does not reach the last entry")
	}

	// The whole message still names every entry, as it did when the entries
	// were joined into one message.
	for _, message := range want {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("the message %q leaves out %q", err.Error(), message)
		}
	}
}

// One entry is one error on its own, as a caller that reads the concrete type
// expects.
func TestOneEntryOfACloudErrorsArrayIsOneError(t *testing.T) {
	const want = "only thing wrong"
	cs := &cloudServer{
		jsonStatus: http.StatusBadRequest,
		jsonBody:   `{"errors":["` + want + `"]}`,
	}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	err = processAndRead(p.CreateFlowData())
	if err == nil {
		t.Fatal("an answer holding one errors entry gave no error")
	}
	var all *CloudRequestErrors
	if errors.As(err, &all) {
		t.Errorf("one entry gave a CloudRequestErrors holding %d", len(all.Errors))
	}
	var one *CloudRequestError
	if !errors.As(err, &one) {
		t.Fatalf("want a CloudRequestError, got %T: %v", err, err)
	}
	if one.Message != want {
		t.Errorf("the message is %q, want %q", one.Message, want)
	}
}
