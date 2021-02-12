package pgo_cli_test

/*
 Copyright 2020 - 2021 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClusterAnnotation(t *testing.T) {
	t.Parallel()

	withNamespace(t, func(namespace func() string) {
		tests := []struct {
			testName    string
			annKey      string
			annVal      string
			annFlag     string
			flags       []string
			deployments []string
		}{
			{
				testName:    "global",
				annKey:      "global",
				annVal:      "here",
				annFlag:     "--annotation",
				flags:       []string{"--pgbouncer"},
				deployments: []string{"global", "global-backrest-shared-repo", "global-pgbouncer"},
			}, {
				testName:    "postgres",
				annKey:      "postgres",
				annVal:      "present",
				annFlag:     "--annotation-postgres",
				flags:       []string{},
				deployments: []string{"postgres"},
			}, {
				testName:    "pgbackrest",
				annKey:      "pgbackrest",
				annVal:      "what",
				annFlag:     "--annotation-pgbackrest",
				flags:       []string{},
				deployments: []string{"pgbackrest-backrest-shared-repo"},
			}, {
				testName:    "pgbouncer",
				annKey:      "pgbouncer",
				annVal:      "aqui",
				annFlag:     "--annotation-pgbouncer",
				flags:       []string{"--pgbouncer"},
				deployments: []string{"pgbouncer-pgbouncer"},
			},
		}

		t.Run("on create", func(t *testing.T) {
			for _, test := range tests {
				// create cluster with flag
				createCMD := []string{"create", "cluster", test.testName, "-n", namespace()}
				createCMD = append(createCMD, test.annFlag+"="+test.annKey+"="+test.annVal)
				createCMD = append(createCMD, test.flags...)
				output, err := pgo(createCMD...).Exec(t)
				defer teardownCluster(t, namespace(), test.testName, time.Now())
				require.NoError(t, err)
				require.Contains(t, output, "created cluster:")

				// wait for cluster
				requireClusterReady(t, namespace(), test.testName, (2 * time.Minute))
				waitPgBouncerReady(t, namespace(), test.testName, time.Minute)

				t.Run("add", func(t *testing.T) {
					t.Run(test.testName, func(t *testing.T) {
						// verify that annotation exists on deployments
						exist, err := TestContext.Kubernetes.CheckAnnotations(namespace(), test.annKey, test.annVal, test.deployments)
						require.NoError(t, err)
						require.True(t, exist)
					})
				})

				t.Run("remove", func(t *testing.T) {
					t.Skip("Bug? annotation in not removed on update")
					updateCMD := []string{"update", "cluster", test.testName, "-n", namespace(), "--no-prompt"}
					updateCMD = append(updateCMD, test.annFlag+"="+test.annKey+"-")
					output, err := pgo(updateCMD...).Exec(t)
					require.NoError(t, err)
					require.Contains(t, output, "updated pgcluster")

					// wait for cluster
					requireClusterReady(t, namespace(), test.testName, (2 * time.Minute))
					waitPgBouncerReady(t, namespace(), test.testName, time.Minute)

					t.Run(test.testName, func(t *testing.T) {
						// verify that annotations don't exist on deployments
						exist, err := TestContext.Kubernetes.CheckAnnotations(namespace(), test.annKey, test.annVal, test.deployments)
						require.NoError(t, err)
						require.False(t, exist)
					})
				})
			}
		})

		t.Run("on update", func(t *testing.T) {
			for _, test := range tests {
				withCluster(t, namespace, func(cluster func() string) {
					// wait for cluster
					requireClusterReady(t, namespace(), cluster(), (2 * time.Minute))
	
					updateCMD := []string{"update", "cluster", cluster(), "-n", namespace(), "--no-prompt"}
					updateCMD = append(updateCMD, test.flags...)
					updateCMD = append(updateCMD, test.annFlag+"="+test.annKey+"="+test.annVal)
					t.Log(updateCMD)
					output, err = pgo(updateCMD...).Exec(t)
					require.NoError(t, err)
					require.Contains(t, output, "updated pgcluster")
	
					// after update wait for cluster to re-create
					waitFor(t, func() bool { return false }, 10*time.Second, time.Second)
					requireClusterReady(t, namespace(), cluster(), (2 * time.Minute))
					waitPgBouncerReady(t, namespace(), cluster(), time.Minute)
	
					// cluster is created without
					t.Run(test.testName, func(t *testing.T) {
						// verify that annotation exists on deployments
						exist, err := TestContext.Kubernetes.CheckAnnotations(namespace(), test.annKey, test.annVal, test.deployments)
						require.NoError(t, err)
						require.True(t, exist)
					})
				}
			}
		})
	})
}
