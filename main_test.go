package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/mock"
	anxcloudDns "go.anx.io/go-anxcloud/pkg/apis/clouddns/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	apimock "github.com/anexia-it/cert-manager-webhook-anexia/mocks"
)

func TestMainSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unit tests")
}

var _ = Describe("Main test", func() {
	var apiClient mock.API
	var ctrl *gomock.Controller
	var k8sClient *testclient.Clientset

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		apiClient = mock.NewMockAPI()
		k8sClient = testclient.NewSimpleClientset()
	})

	Describe("loadConfig", func() {
		It("should return empty config", func() {
			config, err := loadConfig(nil)

			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(BeAssignableToTypeOf(anexiaDNSProviderConfig{}))
			Expect(config.SecretKey).To(BeEmpty())
			Expect(config.SecretRef).To(BeEmpty())
			Expect(config.SecretRefNamespace).To(BeEmpty())
		})

		It("should fail to load none valid json config", func() {
			config, err := loadConfig(&apiextensionsv1.JSON{
				Raw: []byte("everything else but valid json"),
			})

			Expect(err).To(HaveOccurred())
			Expect(config).To(BeAssignableToTypeOf(anexiaDNSProviderConfig{}))
			Expect(config.SecretKey).To(BeEmpty())
			Expect(config.SecretRef).To(BeEmpty())
			Expect(config.SecretRefNamespace).To(BeEmpty())
		})
	})

	Describe("findTXTRecord", func() {
		It("should handle an api list error", func() {
			mockedApiClient := apimock.NewMockAPI(ctrl)

			mockedApiClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("mocked error"))

			txtRecord, err := findTXTRecord(mockedApiClient, context.TODO(), "zone", "anexcloudtest", "foobar")

			Expect(err).To(HaveOccurred())
			Expect(txtRecord).To(BeNil())
		})

		It("should handle no records response", func() {
			notFoundRecord, err := findTXTRecord(apiClient, context.TODO(), "mockZone", "mockRecord", "mockRData")

			Expect(err).ToNot(HaveOccurred())
			Expect(notFoundRecord).To(BeNil())
		})

		It("should handle no duplicate records response", func() {
			apiClient.FakeExisting(
				&anxcloudDns.Record{
					ZoneName: "mockZone",
					Name:     "mockRecord",
					RData:    "mockRData",
				},
			)
			apiClient.FakeExisting(
				&anxcloudDns.Record{
					ZoneName: "mockZone",
					Name:     "mockRecord",
					RData:    "mockRData",
				},
			)

			duplicate, err := findTXTRecord(apiClient, context.TODO(), "mockZone", "mockRecord", "mockRData")

			Expect(err).To(HaveOccurred())
			Expect(duplicate).To(BeNil())
		})

		It("should get single record", func() {
			apiClient.FakeExisting(
				&anxcloudDns.Record{
					ZoneName: "mockZone",
					Name:     "mockRecord",
					RData:    "mockRData",
				},
			)

			record, err := findTXTRecord(apiClient, context.TODO(), "mockZone", "mockRecord", "mockRData")

			Expect(err).ToNot(HaveOccurred())
			Expect(record).ToNot(BeNil())
		})
	})

	Describe("Present", func() {

		It("should make sure a record is present", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, nil
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			err := solver.Present(&v1alpha1.ChallengeRequest{
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should make sure a record is present which is already present", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, nil
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			apiClient.FakeExisting(
				&anxcloudDns.Record{
					ZoneName: "mockZone",
					Name:     "mockRecord",
					RData:    "mockRData",
				},
			)

			err := solver.Present(&v1alpha1.ChallengeRequest{
				// This data does not have to match the faked record from above
				// due to missing filter functionality of the mock api client
				ResolvedZone: "mockZone",
				ResolvedFQDN: "mockRecord.mockZone",
				Key:          "mockRData",
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail to make sure a record is present due to apiClient init error", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, fmt.Errorf("mocked error")
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			err := solver.Present(&v1alpha1.ChallengeRequest{
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).To(HaveOccurred())
		})

		It("should fail to make sure a record is present due to missing secret key", func() {
			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, nil
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			err := solver.Present(&v1alpha1.ChallengeRequest{
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CleanUp", func() {
		It("should make sure a record is cleaned up", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, nil
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			apiClient.FakeExisting(
				&anxcloudDns.Record{
					ZoneName: "mockZone",
					Name:     "mockRecord",
					RData:    "mockRData",
				},
			)

			err := solver.CleanUp(&v1alpha1.ChallengeRequest{
				ResolvedZone: "mockZone",
				ResolvedFQDN: "mockRecord.mockZone",
				Key:          "mockRData",
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail to cleanup non existing record", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, nil
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			err := solver.CleanUp(&v1alpha1.ChallengeRequest{
				ResolvedZone: "mockZone",
				ResolvedFQDN: "mockRecord.mockZone",
				Key:          "mockRData",
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).To(HaveOccurred())
		})

		It("should fail to cleanup record due to apiClient init error", func() {
			createSecret(k8sClient, "foo")

			solver := &anexiaDNSProviderSolver{
				client: k8sClient,
				getEngineClient: func(_ string) (api.API, error) {
					return apiClient, fmt.Errorf("mocked error")
				},
			}

			configByteArray, _ := json.Marshal(anexiaDNSProviderConfig{
				SecretRef:          "foo",
				SecretKey:          "foo",
				SecretRefNamespace: "mock",
			})

			err := solver.CleanUp(&v1alpha1.ChallengeRequest{
				ResolvedZone: "mockZone",
				ResolvedFQDN: "mockRecord.mockZone",
				Key:          "mockRData",
				Config: &apiextensionsv1.JSON{
					Raw: configByteArray,
				},
			})

			Expect(err).To(HaveOccurred())
		})
	})
})

func createSecret(k8sClient *testclient.Clientset, name string) {
	_, err := k8sClient.CoreV1().Secrets("mock").Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			name: []byte("test"),
		},
	}, metav1.CreateOptions{})

	fmt.Println("Error creating mocked secret", err)
}
