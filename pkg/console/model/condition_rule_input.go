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

	meshproto "github.com/apache/dubbo-admin/api/mesh/v1alpha1"
	"github.com/apache/dubbo-admin/pkg/common/constants"
)

type ConditionRuleInput struct {
	ConfigVersion string          `json:"configVersion"`
	Priority      int32           `json:"priority"`
	Enabled       bool            `json:"enabled"`
	Force         bool            `json:"force"`
	Runtime       bool            `json:"runtime"`
	Key           string          `json:"key"`
	Scope         string          `json:"scope"`
	Conditions    json.RawMessage `json:"conditions"`
}

// ToProto keeps v3.0 string conditions and v3.1 structured conditions separate
// so form submissions cannot silently downgrade rule content.
func (i *ConditionRuleInput) ToProto() (*meshproto.ConditionRoute, error) {
	result := &meshproto.ConditionRoute{
		ConfigVersion: i.ConfigVersion,
		Priority:      i.Priority,
		Enabled:       i.Enabled,
		Force:         i.Force,
		Runtime:       i.Runtime,
		Key:           i.Key,
		Scope:         i.Scope,
	}
	if i.ConfigVersion == constants.ConfiguratorVersionV3x1 {
		if err := json.Unmarshal(i.Conditions, &result.ConditionRules); err != nil {
			return nil, err
		}
	} else if err := json.Unmarshal(i.Conditions, &result.Conditions); err != nil {
		return nil, err
	}
	return result, nil
}
