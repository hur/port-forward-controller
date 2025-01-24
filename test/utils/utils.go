/*
Copyright 2025.

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

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
)

const (
	prometheusOperatorVersion = "v0.77.1"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.16.0"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"

	unifiControllerVersion   = "v9.0.108"
	mikrotikContainerVersion = "7.17"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return string(output), nil
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "create", "-f", url)
	_, err := Run(cmd)
	return err
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// by verifying the existence of key CRDs related to Prometheus.
func IsPrometheusCRDsInstalled() bool {
	// List of common Prometheus CRDs
	prometheusCRDs := []string{
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
	}

	cmd := exec.Command("kubectl", "get", "crds", "-o", "custom-columns=NAME:.metadata.name")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	crdList := GetNonEmptyLines(output)
	for _, crd := range prometheusCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled() bool {
	// List of common Cert Manager CRDs
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"certificaterequests.cert-manager.io",
		"orders.acme.cert-manager.io",
		"challenges.acme.cert-manager.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range certManagerCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		_, err := out.WriteString(strings.TrimPrefix(scanner.Text(), prefix))
		if err != nil {
			return err
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err := out.WriteString("\n"); err != nil {
			return err
		}
	}

	_, err = out.Write(content[idx+len(target):])
	if err != nil {
		return err
	}
	// false positive
	// nolint:gosec
	return os.WriteFile(filename, out.Bytes(), 0644)
}

const unifiControllerSpecTmpl = `
---
apiVersion: v1
kind: Service
metadata:
  name: unifi
  labels:
    app.kubernetes.io/instance: unifi
    app.kubernetes.io/name: unifi
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: controller
    protocol: TCP
    name: controller
  - port: 10001
    targetPort: discovery
    protocol: UDP
    name: discovery
  - port: 8443
    targetPort: http
    protocol: TCP
    name: http
  - port: 6789
    targetPort: speedtest
    protocol: TCP
    name: speedtest
  - port: 3478
    targetPort: stun
    protocol: UDP
    name: stun
  - port: 5514
    targetPort: syslog
    protocol: UDP
    name: syslog
  selector:
    app.kubernetes.io/name: unifi
    app.kubernetes.io/instance: unifi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: unifi
  labels:
    app.kubernetes.io/instance: unifi
    app.kubernetes.io/name: unifi
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: unifi
      app.kubernetes.io/instance: unifi
  template:
    metadata:
      labels:
        app.kubernetes.io/name: unifi
        app.kubernetes.io/instance: unifi
    spec:
      serviceAccountName: default
      automountServiceAccountToken: true
      securityContext:
        fsGroup: 999
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      containers:
        - name: unifi
          image: "jacobalberty/unifi:%s"
          imagePullPolicy: IfNotPresent
          env:
            - name: JVM_INIT_HEAP_SIZE
              value: null
            - name: JVM_MAX_HEAP_SIZE
              value: 1024M
            - name: RUNAS_UID0
              value: "false"
            - name: TZ
              value: UTC
            - name: UNIFI_GID
              value: "999"
            - name: UNIFI_STDOUT
              value: "true"
            - name: UNIFI_UID
              value: "999"
          ports:
            - name: controller
              containerPort: 8080
              protocol: TCP
            - name: discovery
              containerPort: 10001
              protocol: UDP
            - name: http
              containerPort: 8443
              protocol: TCP
            - name: speedtest
              containerPort: 6789
              protocol: TCP
            - name: stun
              containerPort: 3478
              protocol: UDP
            - name: syslog
              containerPort: 5514
              protocol: UDP
          livenessProbe:
            tcpSocket:
              port: 8443
            initialDelaySeconds: 0
            failureThreshold: 3
            timeoutSeconds: 1
            periodSeconds: 10
          readinessProbe:
            tcpSocket:
              port: 8443
            initialDelaySeconds: 0
            failureThreshold: 3
            timeoutSeconds: 1
            periodSeconds: 10
          startupProbe:
            tcpSocket:
              port: 8443
            initialDelaySeconds: 0
            failureThreshold: 30
            timeoutSeconds: 1
            periodSeconds: 5
`

func InstallUnifiController() (string, error) {
	unifiControllerSpec := fmt.Sprintf(unifiControllerSpecTmpl, unifiControllerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(unifiControllerSpec)
	if _, err := Run(cmd); err != nil {
		return "", err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/unifi",
		"--for", "condition=Available",
		"--namespace", "default",
		"--timeout", "5m",
	)
	_, err := Run(cmd)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("kubectl", "get", "service", "unifi", "-o", "jsonpath='{.spec.clusterIP}'").Output()
	// HACK: out is wrapped in single quotes and contains trailing newline
	res := ""
	if len(out) > 3 {
		res = string(out)[1:][:len(out)-3]
	}
	return res, err
}

func UninstallUnifiController() {
	unifiControllerSpec := fmt.Sprintf(unifiControllerSpecTmpl, unifiControllerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", "-")
	cmd.Stdin = strings.NewReader(unifiControllerSpec)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

}

const mikrotikContainerSpecTmpl = `
---
apiVersion: v1
kind: Service
metadata:
  name: mikrotik
  labels:
    app.kubernetes.io/instance: mikrotik
    app.kubernetes.io/name: mikrotik
spec:
  type: ClusterIP
  ports:
  - port: 8728
    targetPort: controller
    protocol: TCP
    name: controller
  selector:
    app.kubernetes.io/name: mikrotik
    app.kubernetes.io/instance: mikrotik
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mikrotik
  labels:
    app.kubernetes.io/instance: mikrotik
    app.kubernetes.io/name: mikrotik
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: mikrotik
      app.kubernetes.io/instance: mikrotik
  template:
    metadata:
      labels:
        app.kubernetes.io/name: mikrotik
        app.kubernetes.io/instance: mikrotik
    spec:
      serviceAccountName: default
      automountServiceAccountToken: true
      securityContext:
        fsGroup: 999
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      containers:
        - name: mikrotik
          image: "jacobalberty/unifi:%s"
          imagePullPolicy: IfNotPresent
          ports:
            - name: controller
              containerPort: 8728
              protocol: TCP
`

func InstallMikrotikContainer() (string, error) {
	mikrotikContainerSpec := fmt.Sprintf(mikrotikContainerSpecTmpl, mikrotikContainerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(mikrotikContainerSpec)
	if _, err := Run(cmd); err != nil {
		return "", err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/mikrotik",
		"--for", "condition=Available",
		"--namespace", "default",
		"--timeout", "5m",
	)
	_, err := Run(cmd)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("kubectl", "get", "service", "mikrotik", "-o", "jsonpath='{.spec.clusterIP}'").Output()
	// HACK: out is wrapped in single quotes and contains trailing newline
	res := ""
	if len(out) > 3 {
		res = string(out)[1:][:len(out)-3]
	}
	return res, err
}
