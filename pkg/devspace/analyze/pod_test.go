package analyze

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/devspace-cloud/devspace/pkg/devspace/kubectl"
	fakekube "github.com/devspace-cloud/devspace/pkg/devspace/kubectl/testing"
	"github.com/devspace-cloud/devspace/pkg/util/log"
	"github.com/mgutz/ansi"
	"gotest.tools/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type podTestCase struct {
	name string

	noWait bool
	pod    k8sv1.Pod

	updatedPod *k8sv1.Pod

	expectedProblems []string
	expectedErr      string
}

func TestPods(t *testing.T) {
	testCases := []podTestCase{
		podTestCase{
			name: "Wait for pod in creation",
			pod: k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason: kubectl.WaitStatus[0],
				},
			},
			updatedPod: &k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason:    "Running",
					StartTime: &metav1.Time{Time: time.Now().Add(-MinimumPodAge * 2)},
				},
			},
		},
		podTestCase{
			name: "Wait for pod in initialization",
			pod: k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason: "Init: something",
				},
			},
			updatedPod: &k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason:    "Running",
					StartTime: &metav1.Time{Time: time.Now().Add(-MinimumPodAge * 2)},
				},
			},
		},
		podTestCase{
			name: "Wait for minimalPodAge to pass",
			pod: k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason:    "Running",
					StartTime: &metav1.Time{Time: time.Now()},
				},
			},
			updatedPod: &k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason:    "Running",
					StartTime: &metav1.Time{Time: time.Now().Add(-MinimumPodAge * 2)},
				},
			},
		},
		podTestCase{
			name:   "Analyze pod with many problems",
			noWait: true,
			pod: k8sv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testPod",
				},
				Status: k8sv1.PodStatus{
					Reason: "Error",
					ContainerStatuses: []k8sv1.ContainerStatus{
						k8sv1.ContainerStatus{
							Ready:        true,
							RestartCount: 1,
							LastTerminationState: k8sv1.ContainerState{
								Terminated: &k8sv1.ContainerStateTerminated{
									FinishedAt: metav1.Time{Time: time.Now().Add(-IgnoreRestartsSince * 2)},
									ExitCode:   int32(1),
									Message:    "someMessage",
									Reason:     "someReason",
								},
							},
						},
						k8sv1.ContainerStatus{
							State: k8sv1.ContainerState{
								Terminated: &k8sv1.ContainerStateTerminated{
									FinishedAt: metav1.Time{Time: time.Now().Add(-IgnoreRestartsSince * 2)},
									Message:    "someMessage2",
									Reason:     "someReason2",
									ExitCode:   int32(2),
								},
							},
						},
					},
					InitContainerStatuses: []k8sv1.ContainerStatus{
						k8sv1.ContainerStatus{
							Ready: true,
						},
						k8sv1.ContainerStatus{
							State: k8sv1.ContainerState{
								Waiting: &k8sv1.ContainerStateWaiting{
									Message: "someMessage3",
									Reason:  "someReason3",
								},
							},
						},
					},
				},
			},
			expectedProblems: []string{
				fmt.Sprintf("Pod %s:", ansi.Color("testPod", "white+b")),
				fmt.Sprintf("    Status: %s", ansi.Color("Init:0/0", "yellow+b")),
				fmt.Sprintf("    Container: %s/2 running", ansi.Color("1", "red+b")),
				"    Problems: ",
				fmt.Sprintf("      - Container: %s", ansi.Color("", "white+b")),
				fmt.Sprintf("        Status: %s (reason: %s)", ansi.Color("Terminated", "red+b"), ansi.Color("someReason2", "red+b")),
				fmt.Sprintf("        Message: %s", ansi.Color("someMessage2", "white+b")),
				fmt.Sprintf("        Last Execution Log: \n%s", ansi.Color("ContainerLogs", "red")),
				"    InitContainer Problems: ",
				fmt.Sprintf("      - Container: %s", ansi.Color("", "white+b")),
				fmt.Sprintf("        Status: %s (reason: %s)", ansi.Color("Waiting", "red+b"), ansi.Color("someReason3", "red+b")),
				fmt.Sprintf("        Message: %s", ansi.Color("someMessage3", "white+b")),
			},
		},
	}

	for _, testCase := range testCases {
		namespace := "testns"
		kubeClient := &fakekube.Client{
			Client: fake.NewSimpleClientset(),
		}
		kubeClient.Client.CoreV1().Namespaces().Create(&k8sv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})
		kubeClient.Client.CoreV1().Pods(namespace).Create(&testCase.pod)

		analyzer := &analyzer{
			client: kubeClient,
			log:    log.Discard,
		}

		go func() {
			time.Sleep(time.Second / 2)
			if testCase.updatedPod != nil {
				kubeClient.Client.CoreV1().Pods(namespace).Update(testCase.updatedPod)
			}
		}()

		problems, err := analyzer.pods(namespace, testCase.noWait)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}

		lineWithTimestamp := regexp.MustCompile("(?m)[\r\n]+^.*ago.*$")
		result := ""
		if len(problems) > 0 {
			result = lineWithTimestamp.ReplaceAllString(problems[0], "")
		}
		expectedString := ""
		if len(testCase.expectedProblems) > 0 {
			expectedString = paddingLeft + strings.Join(testCase.expectedProblems, paddingLeft+"\n") + "\n"
		}
		assert.Equal(t, result, expectedString, "Unexpected problem list in testCase %s", testCase.name)
	}

	/*_, err = kubeClient.Client.CoreV1().Pods("testNS").Update(&k8sv1.Pod{
		Status: k8sv1.PodStatus{
			Reason: "Running",
			ContainerStatuses: []k8sv1.ContainerStatus{
				k8sv1.ContainerStatus{
					RestartCount: 1,
					LastTerminationState: k8sv1.ContainerState{
						Terminated: &k8sv1.ContainerStateTerminated{
							FinishedAt: metav1.Time{Time: timeNow},
							ExitCode:   1,
							Message:    "This container terminated. Happy debugging!",
							Reason:     "Stopped",
						},
					},
					Ready: false,
					State: k8sv1.ContainerState{
						Waiting: &k8sv1.ContainerStateWaiting{
							Reason:  "Restarting",
							Message: "Restarting after this container hit an error.",
						},
					},
				},
			},
		},
	})
	assert.NilError(t, err, "Error updating pod")

	expectedPodProblem := &podProblem{
		Status:         "Restarting",
		ContainerTotal: 1,
		ContainerProblems: []*containerProblem{
			&containerProblem{
				Name:           "",
				Waiting:        true,
				Reason:         "Restarting",
				Message:        "Restarting after this container hit an error.",
				Restarts:       1,
				LastRestart:    time.Since(timeNow),
				LastExitReason: "Stopped",
				LastExitCode:   1,
				LastMessage:    "This container terminated. Happy debugging!",
			},
		},
	}*/
}
