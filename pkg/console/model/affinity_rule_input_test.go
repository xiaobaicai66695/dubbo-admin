/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meshproto "github.com/apache/dubbo-admin/api/mesh/v1alpha1"
)

func TestAffinityRuleInputUsesPublicAffinityAwareField(t *testing.T) {
	input := &AffinityRuleInput{
		ConfigVersion: "v3.1",
		Scope:         "application",
		Key:           "demo",
		Runtime:       true,
		Enabled:       true,
		AffinityAware: &meshproto.AffinityAware{Key: "region", Ratio: 80},
	}

	spec := input.ToProto()
	assert.Equal(t, input.AffinityAware, spec.Affinity)

	raw, err := json.Marshal(GenAffinityRuleResp(spec))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "affinityAware")
	assert.NotContains(t, string(raw), `"affinity"`)
}
