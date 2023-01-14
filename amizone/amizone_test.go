package amizone_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/ditsuke/go-amizone/amizone"
	"github.com/ditsuke/go-amizone/amizone/internal/mock"
	"github.com/ditsuke/go-amizone/amizone/models"
	. "github.com/onsi/gomega"
	"gopkg.in/h2non/gock.v1"
)

// @todo: implement test cases to test behavior when:
// - Amizone is not reachable
// - Amizone is reachable but login fails (invalid credentials, etc?)
func TestNewClient(t *testing.T) {
	g := NewGomegaWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	err := mock.GockRegisterLoginPage()
	g.Expect(err).ToNot(HaveOccurred(), "failed to register login page mock")
	err = mock.GockRegisterLoginRequest()
	g.Expect(err).ToNot(HaveOccurred(), "failed to register login request mock")

	jar, err := cookiejar.New(nil)
	g.Expect(err).ToNot(HaveOccurred(), "failed to create cookie jar")

	httpClient := &http.Client{Jar: jar}
	gock.InterceptClient(httpClient)

	c := amizone.Credentials{
		Username: mock.ValidUser,
		Password: mock.ValidPass,
	}

	client, err := amizone.NewClient(c, httpClient)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(client).ToNot(BeNil())
}

// What are your expectations of this function?
// Login? No. That's not its responsibility.
// What we do expect is:
// It makes a request as the amizone client mocked would
// And then it retrieves the attendance record from the test page as it exists.
// Cases: Right record with the login mocked, no record with no login.
func TestAmizoneClient_GetAttendance(t *testing.T) {
	g := NewGomegaWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	nonLoggedInClient := getNonLoggedInClient(g)
	loggedInClient := getLoggedInClient(g)

	gock.Clean()

	testCases := []struct {
		name              string
		amizoneClient     *amizone.Client
		setup             func(g *WithT)
		attendanceMatcher func(g *WithT, attendance models.AttendanceRecords)
		errorMatcher      func(g *WithT, err error)
	}{
		{
			name:          "Logged in, expecting retrieval",
			amizoneClient: loggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterHomePageLoggedIn()
				g.Expect(err).ToNot(HaveOccurred())
			},
			attendanceMatcher: func(g *WithT, attendance models.AttendanceRecords) {
				g.Expect(len(attendance)).To(Equal(8))
			},
			errorMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:          "Not logged in, expecting no retrieval",
			amizoneClient: nonLoggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterUnauthenticatedGet("/Home")
				g.Expect(err).ToNot(HaveOccurred())
			},
			attendanceMatcher: func(g *WithT, attendance models.AttendanceRecords) {
				g.Expect(attendance).To(BeEmpty())
			},
			errorMatcher: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("not logged in"))
			},
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			t.Cleanup(setupNetworking)

			c.setup(g)

			attendance, err := c.amizoneClient.GetAttendance()
			c.attendanceMatcher(g, attendance)
			c.errorMatcher(g, err)
		})
	}
}

func TestClient_GetSemesters(t *testing.T) {
	g := NewGomegaWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	loggedInClient := getLoggedInClient(g)
	nonLoggedInClient := getNonLoggedInClient(g)

	testCases := []struct {
		name             string
		client           *amizone.Client
		setup            func(g *WithT)
		semestersMatcher func(g *WithT, semesters models.SemesterList)
		errMatcher       func(g *WithT, err error)
	}{
		{
			name:   "client is logged in and amizone returns a (mock) courses page",
			client: loggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterCurrentCoursesPage()
				g.Expect(err).ToNot(HaveOccurred())
			},
			semestersMatcher: func(g *WithT, semesters models.SemesterList) {
				g.Expect(semesters).To(HaveLen(4))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:   "client is not logged in and amizone returns the login page",
			client: nonLoggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterLoginPage()
				g.Expect(err).ToNot(HaveOccurred())
			},
			semestersMatcher: func(g *WithT, semesters models.SemesterList) {
				g.Expect(semesters).To(HaveLen(0))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Cleanup(setupNetworking)
			testCase.setup(g)

			semesters, err := testCase.client.GetSemesters()
			testCase.errMatcher(g, err)
			testCase.semestersMatcher(g, semesters)
		})
	}
}

func TestClient_GetCourses(t *testing.T) {
	g := NewWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	loggedInClient := getLoggedInClient(g)
	nonLoggedInClient := getNonLoggedInClient(g)

	testCases := []struct {
		name           string
		client         *amizone.Client
		semesterRef    string
		setup          func(g *WithT)
		coursesMatcher func(g *WithT, courses models.Courses)
		errMatcher     func(g *WithT, err error)
	}{
		{
			name:        "amizone client is logged in, we ask for semester 1, return mock courses page on expected POST",
			client:      loggedInClient,
			semesterRef: "1",
			setup: func(g *WithT) {
				err := mock.GockRegisterSemesterCoursesRequest("1")
				g.Expect(err).ToNot(HaveOccurred())
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(8))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:        "amizone client is logged in, we ask for semester 2, return mock courses page on expected POST",
			client:      loggedInClient,
			semesterRef: "2",
			setup: func(g *WithT) {
				err := mock.GockRegisterSemesterCoursesRequest("2")
				g.Expect(err).ToNot(HaveOccurred())
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(8))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:        "amizone client is not logged in, returns login page on request",
			client:      nonLoggedInClient,
			semesterRef: "3",
			setup: func(g *WithT) {
				//err := mock.GockRegisterLoginPage()
				//g.Expect(err).ToNot(HaveOccurred())
				err := mock.GockRegisterUnauthenticatedGet("/")
				g.Expect(err).ToNot(HaveOccurred())
				mock.GockRegisterUnauthenticatedPost("/CourseListSemWise", url.Values{"sem": []string{"3"}}.Encode(), strings.NewReader("<no></no>"))
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(0))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).ToNot(ContainSubstring(amizone.ErrFailedToVisitPage))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Cleanup(setupNetworking)
			testCase.setup(g)

			courses, err := testCase.client.GetCourses(testCase.semesterRef)
			testCase.errMatcher(g, err)
			testCase.coursesMatcher(g, courses)
		})
	}
}

func TestClient_GetCurrentCourses(t *testing.T) {
	g := NewWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	loggedInClient := getLoggedInClient(g)
	nonLoggedInClient := getNonLoggedInClient(g)

	testCases := []struct {
		name           string
		client         *amizone.Client
		setup          func(g *WithT)
		coursesMatcher func(g *WithT, courses models.Courses)
		errMatcher     func(g *WithT, err error)
	}{
		{
			name:   "amizone client is logged in and returns the (mock) courses page",
			client: loggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterCurrentCoursesPage()
				g.Expect(err).ToNot(HaveOccurred())
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(8))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:   "amizone client is logged is and returns the (mock) sem-wise courses page",
			client: loggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterSemWiseCoursesPage()
				g.Expect(err).ToNot(HaveOccurred())
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(8))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:   "amizone client is not logged in and returns the login page",
			client: nonLoggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterUnauthenticatedGet("/")
				g.Expect(err).ToNot(HaveOccurred())
			},
			coursesMatcher: func(g *WithT, courses models.Courses) {
				g.Expect(courses).To(HaveLen(0))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Cleanup(setupNetworking)
			testCase.setup(g)

			courses, err := testCase.client.GetCurrentCourses()
			testCase.errMatcher(g, err)
			testCase.coursesMatcher(g, courses)
		})
	}
}

func TestClient_GetProfile(t *testing.T) {
	g := NewWithT(t)

	setupNetworking()
	t.Cleanup(teardown)

	loggedInClient := getLoggedInClient(g)
	nonLoggedInClient := getNonLoggedInClient(g)

	testCases := []struct {
		name           string
		client         *amizone.Client
		setup          func(g *WithT)
		profileMatcher func(g *WithT, profile *models.Profile)
		errMatcher     func(g *WithT, err error)
	}{
		{
			name:   "amizone client logged in and returns the (mock) profile page",
			client: loggedInClient,
			setup: func(g *WithT) {
				err := mock.GockRegisterProfilePage()
				g.Expect(err).ToNot(HaveOccurred())
			},
			profileMatcher: func(g *WithT, profile *models.Profile) {
				g.Expect(profile).To(Equal(&models.Profile{
					Name:               mock.StudentName,
					EnrollmentNumber:   mock.StudentEnrollmentNumber,
					EnrollmentValidity: mock.StudentIDValidity.Time(),
					DateOfBirth:        mock.StudentDOB.Time(),
					Batch:              mock.StudentBatch,
					Program:            mock.StudentProgram,
					BloodGroup:         mock.StudentBloodGroup,
					IDCardNumber:       mock.StudentIDCardNumber,
					UUID:               mock.StudentUUID,
				}))
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			name:   "amizone client is not logged in and returns the login page",
			client: nonLoggedInClient,
			setup: func(g *WithT) {
				_ = mock.GockRegisterUnauthenticatedGet("/IDCard")
			},
			profileMatcher: func(g *WithT, profile *models.Profile) {
				g.Expect(profile).To(BeNil())
			},
			errMatcher: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("not logged in"))
			},
		},
	}

	for _, testCases := range testCases {
		t.Run(testCases.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Cleanup(setupNetworking)
			testCases.setup(g)

			profile, err := testCases.client.GetProfile()
			testCases.errMatcher(g, err)
			testCases.profileMatcher(g, profile)
		})
	}
}


// setupNetworking tears down any existing network mocks and sets up gock anew to intercept network
// calls and disable real network calls.
func setupNetworking() {
	// tear everything all routes down
	teardown()
	gock.Intercept()
	gock.DisableNetworking()
}

// teardown disables all networking restrictions and mock routes registered with gock for unit testing.
func teardown() {
	gock.Clean()
	gock.Off()
	gock.EnableNetworking()
}

func getNonLoggedInClient(g *GomegaWithT) *amizone.Client {
	client, err := amizone.NewClient(amizone.Credentials{}, nil)
	g.Expect(err).ToNot(HaveOccurred())
	return client
}

func getLoggedInClient(g *GomegaWithT) *amizone.Client {
	err := mock.GockRegisterLoginPage()
	g.Expect(err).ToNot(HaveOccurred(), "failed to register mock login page")
	err = mock.GockRegisterLoginRequest()
	g.Expect(err).ToNot(HaveOccurred(), "failed to register mock login request")

	client, err := amizone.NewClient(amizone.Credentials{
		Username: mock.ValidUser,
		Password: mock.ValidPass,
	}, nil)
	g.Expect(err).ToNot(HaveOccurred(), "failed to setup mock logged-in client")
	return client
}
