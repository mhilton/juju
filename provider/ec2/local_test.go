// Copyright 2011, 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ec2_test

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/juju/errors"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils"
	"github.com/juju/utils/set"
	"gopkg.in/amz.v3/aws"
	amzec2 "gopkg.in/amz.v3/ec2"
	"gopkg.in/amz.v3/ec2/ec2test"
	"gopkg.in/amz.v3/s3/s3test"
	gc "gopkg.in/check.v1"
	goyaml "gopkg.in/yaml.v1"

	"github.com/juju/juju/constraints"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/bootstrap"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/configstore"
	"github.com/juju/juju/environs/imagemetadata"
	"github.com/juju/juju/environs/jujutest"
	"github.com/juju/juju/environs/simplestreams"
	envtesting "github.com/juju/juju/environs/testing"
	"github.com/juju/juju/environs/tools"
	"github.com/juju/juju/feature"
	"github.com/juju/juju/instance"
	"github.com/juju/juju/juju/arch"
	"github.com/juju/juju/juju/testing"
	"github.com/juju/juju/network"
	"github.com/juju/juju/provider/common"
	"github.com/juju/juju/provider/ec2"
	coretesting "github.com/juju/juju/testing"
	"github.com/juju/juju/utils/ssh"
	"github.com/juju/juju/version"
)

type ProviderSuite struct {
	coretesting.BaseSuite
}

var _ = gc.Suite(&ProviderSuite{})

var localConfigAttrs = coretesting.FakeConfig().Merge(coretesting.Attrs{
	"name":           "sample",
	"type":           "ec2",
	"region":         "test",
	"control-bucket": "test-bucket",
	"access-key":     "x",
	"secret-key":     "x",
	"agent-version":  coretesting.FakeVersionNumber.String(),
})

func registerLocalTests() {
	// N.B. Make sure the region we use here
	// has entries in the images/query txt files.
	aws.Regions["test"] = aws.Region{
		Name: "test",
	}

	gc.Suite(&localServerSuite{})
	gc.Suite(&localLiveSuite{})
	gc.Suite(&localNonUSEastSuite{})
}

// localLiveSuite runs tests from LiveTests using a fake
// EC2 server that runs within the test process itself.
type localLiveSuite struct {
	LiveTests
	srv                localServer
	restoreEC2Patching func()
}

func (t *localLiveSuite) SetUpSuite(c *gc.C) {
	// Upload arches that ec2 supports; add to this
	// as ec2 coverage expands.
	t.UploadArches = []string{arch.AMD64, arch.I386}
	t.TestConfig = localConfigAttrs
	t.restoreEC2Patching = patchEC2ForTesting()
	t.srv.createRootDisks = true
	t.srv.startServer(c)
	t.LiveTests.SetUpSuite(c)
}

func (t *localLiveSuite) TearDownSuite(c *gc.C) {
	t.LiveTests.TearDownSuite(c)
	t.srv.stopServer(c)
	t.restoreEC2Patching()
}

// localServer represents a fake EC2 server running within
// the test process itself.
type localServer struct {
	// createRootDisks is used to decide whether or not
	// the ec2test server will create root disks for
	// instances.
	createRootDisks bool

	ec2srv *ec2test.Server
	s3srv  *s3test.Server
	config *s3test.Config
}

func (srv *localServer) startServer(c *gc.C) {
	var err error
	srv.ec2srv, err = ec2test.NewServer()
	if err != nil {
		c.Fatalf("cannot start ec2 test server: %v", err)
	}
	srv.ec2srv.SetCreateRootDisks(srv.createRootDisks)
	srv.s3srv, err = s3test.NewServer(srv.config)
	if err != nil {
		c.Fatalf("cannot start s3 test server: %v", err)
	}
	aws.Regions["test"] = aws.Region{
		Name:                 "test",
		EC2Endpoint:          srv.ec2srv.URL(),
		S3Endpoint:           srv.s3srv.URL(),
		S3LocationConstraint: true,
	}
	srv.addSpice(c)

	zones := make([]amzec2.AvailabilityZoneInfo, 3)
	zones[0].Region = "test"
	zones[0].Name = "test-available"
	zones[0].State = "available"
	zones[1].Region = "test"
	zones[1].Name = "test-impaired"
	zones[1].State = "impaired"
	zones[2].Region = "test"
	zones[2].Name = "test-unavailable"
	zones[2].State = "unavailable"
	srv.ec2srv.SetAvailabilityZones(zones)
	srv.ec2srv.SetInitialInstanceState(ec2test.Pending)
}

// addSpice adds some "spice" to the local server
// by adding state that may cause tests to fail.
func (srv *localServer) addSpice(c *gc.C) {
	states := []amzec2.InstanceState{
		ec2test.ShuttingDown,
		ec2test.Terminated,
		ec2test.Stopped,
	}
	for _, state := range states {
		srv.ec2srv.NewInstances(1, "m1.small", "ami-a7f539ce", state, nil)
	}
}

func (srv *localServer) stopServer(c *gc.C) {
	srv.ec2srv.Quit()
	srv.s3srv.Quit()
	// Clear out the region because the server address is
	// no longer valid.
	delete(aws.Regions, "test")
}

// localServerSuite contains tests that run against a fake EC2 server
// running within the test process itself.  These tests can test things that
// would be unreasonably slow or expensive to test on a live Amazon server.
// It starts a new local ec2test server for each test.  The server is
// accessed by using the "test" region, which is changed to point to the
// network address of the local server.
type localServerSuite struct {
	coretesting.BaseSuite
	jujutest.Tests
	srv                localServer
	restoreEC2Patching func()
}

func (t *localServerSuite) SetUpSuite(c *gc.C) {
	// Upload arches that ec2 supports; add to this
	// as ec2 coverage expands.
	t.UploadArches = []string{arch.AMD64, arch.I386}
	t.TestConfig = localConfigAttrs
	t.restoreEC2Patching = patchEC2ForTesting()
	t.BaseSuite.SetUpSuite(c)
	t.srv.createRootDisks = true
}

func (t *localServerSuite) TearDownSuite(c *gc.C) {
	t.BaseSuite.TearDownSuite(c)
	t.restoreEC2Patching()
}

func (t *localServerSuite) SetUpTest(c *gc.C) {
	t.PatchValue(&version.Current, version.Binary{
		Number: coretesting.FakeVersionNumber,
		Series: coretesting.FakeDefaultSeries,
		Arch:   arch.AMD64,
	})
	t.BaseSuite.SetUpTest(c)
	t.SetFeatureFlags(feature.AddressAllocation)
	t.srv.startServer(c)
	t.Tests.SetUpTest(c)
}

func (t *localServerSuite) TearDownTest(c *gc.C) {
	t.Tests.TearDownTest(c)
	t.srv.stopServer(c)
	t.BaseSuite.TearDownTest(c)
}

func (t *localServerSuite) prepareEnviron(c *gc.C) environs.NetworkingEnviron {
	env := t.Prepare(c)
	netenv, supported := environs.SupportsNetworking(env)
	c.Assert(supported, jc.IsTrue)
	return netenv
}

func (t *localServerSuite) TestBootstrapInstanceUserDataAndState(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	// check that StateServerInstances returns the id of the bootstrap machine.
	instanceIds, err := env.StateServerInstances()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(instanceIds, gc.HasLen, 1)

	insts, err := env.AllInstances()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(insts, gc.HasLen, 1)
	c.Check(insts[0].Id(), gc.Equals, instanceIds[0])

	// check that the user data is configured to start zookeeper
	// and the machine and provisioning agents.
	// check that the user data is configured to only configure
	// authorized SSH keys and set the log output; everything
	// else happens after the machine is brought up.
	inst := t.srv.ec2srv.Instance(string(insts[0].Id()))
	c.Assert(inst, gc.NotNil)
	addresses, err := insts[0].Addresses()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(addresses, gc.Not(gc.HasLen), 0)
	userData, err := utils.Gunzip(inst.UserData)
	c.Assert(err, jc.ErrorIsNil)
	c.Logf("first instance: UserData: %q", userData)
	var userDataMap map[interface{}]interface{}
	err = goyaml.Unmarshal(userData, &userDataMap)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(userDataMap, jc.DeepEquals, map[interface{}]interface{}{
		"output": map[interface{}]interface{}{
			"all": "| tee -a /var/log/cloud-init-output.log",
		},
		"ssh_authorized_keys": splitAuthKeys(env.Config().AuthorizedKeys()),
		"runcmd": []interface{}{
			"set -xe",
			"install -D -m 644 /dev/null '/etc/init/juju-clean-shutdown.conf'",
			"printf '%s\\n' '\nauthor \"Juju Team <juju@lists.ubuntu.com>\"\ndescription \"Stop all network interfaces on shutdown\"\nstart on runlevel [016]\ntask\nconsole output\n\nexec /sbin/ifdown -a -v --force\n' > '/etc/init/juju-clean-shutdown.conf'",
			"install -D -m 644 /dev/null '/var/lib/juju/nonce.txt'",
			"printf '%s\\n' 'user-admin:bootstrap' > '/var/lib/juju/nonce.txt'",
		},
	})

	// check that a new instance will be started with a machine agent
	inst1, hc := testing.AssertStartInstance(c, env, "1")
	c.Check(*hc.Arch, gc.Equals, "amd64")
	c.Check(*hc.Mem, gc.Equals, uint64(1740))
	c.Check(*hc.CpuCores, gc.Equals, uint64(1))
	c.Assert(*hc.CpuPower, gc.Equals, uint64(100))
	inst = t.srv.ec2srv.Instance(string(inst1.Id()))
	c.Assert(inst, gc.NotNil)
	userData, err = utils.Gunzip(inst.UserData)
	c.Assert(err, jc.ErrorIsNil)
	c.Logf("second instance: UserData: %q", userData)
	userDataMap = nil
	err = goyaml.Unmarshal(userData, &userDataMap)
	c.Assert(err, jc.ErrorIsNil)
	CheckPackage(c, userDataMap, "curl", true)
	CheckPackage(c, userDataMap, "mongodb-server", false)
	CheckScripts(c, userDataMap, "jujud bootstrap-state", false)
	CheckScripts(c, userDataMap, "/var/lib/juju/agents/machine-1/agent.conf", true)
	// TODO check for provisioning agent

	err = env.Destroy()
	c.Assert(err, jc.ErrorIsNil)

	_, err = env.StateServerInstances()
	c.Assert(err, gc.Equals, environs.ErrNotBootstrapped)
}

// splitAuthKeys splits the given authorized keys
// into the form expected to be found in the
// user data.
func splitAuthKeys(keys string) []interface{} {
	slines := strings.FieldsFunc(keys, func(r rune) bool {
		return r == '\n'
	})
	var lines []interface{}
	for _, line := range slines {
		lines = append(lines, ssh.EnsureJujuComment(strings.TrimSpace(line)))
	}
	return lines
}

func (t *localServerSuite) TestInstanceStatus(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)
	t.srv.ec2srv.SetInitialInstanceState(ec2test.Terminated)
	inst, _ := testing.AssertStartInstance(c, env, "1")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(inst.Status(), gc.Equals, "terminated")
}

func (t *localServerSuite) TestStartInstanceHardwareCharacteristics(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)
	_, hc := testing.AssertStartInstance(c, env, "1")
	c.Check(*hc.Arch, gc.Equals, "amd64")
	c.Check(*hc.Mem, gc.Equals, uint64(1740))
	c.Check(*hc.CpuCores, gc.Equals, uint64(1))
	c.Assert(*hc.CpuPower, gc.Equals, uint64(100))
}

func (t *localServerSuite) TestStartInstanceAvailZone(c *gc.C) {
	inst, err := t.testStartInstanceAvailZone(c, "test-available")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(ec2.InstanceEC2(inst).AvailZone, gc.Equals, "test-available")
}

func (t *localServerSuite) TestStartInstanceAvailZoneImpaired(c *gc.C) {
	_, err := t.testStartInstanceAvailZone(c, "test-impaired")
	c.Assert(err, gc.ErrorMatches, `availability zone "test-impaired" is impaired`)
}

func (t *localServerSuite) TestStartInstanceAvailZoneUnknown(c *gc.C) {
	_, err := t.testStartInstanceAvailZone(c, "test-unknown")
	c.Assert(err, gc.ErrorMatches, `invalid availability zone "test-unknown"`)
}

func (t *localServerSuite) testStartInstanceAvailZone(c *gc.C, zone string) (instance.Instance, error) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	params := environs.StartInstanceParams{Placement: "zone=" + zone}
	result, err := testing.StartInstanceWithParams(env, "1", params, nil)
	if err != nil {
		return nil, err
	}
	return result.Instance, nil
}

func (t *localServerSuite) TestGetAvailabilityZones(c *gc.C) {
	var resultZones []amzec2.AvailabilityZoneInfo
	var resultErr error
	t.PatchValue(ec2.EC2AvailabilityZones, func(e *amzec2.EC2, f *amzec2.Filter) (*amzec2.AvailabilityZonesResp, error) {
		resp := &amzec2.AvailabilityZonesResp{
			Zones: append([]amzec2.AvailabilityZoneInfo{}, resultZones...),
		}
		return resp, resultErr
	})
	env := t.Prepare(c).(common.ZonedEnviron)

	resultErr = fmt.Errorf("failed to get availability zones")
	zones, err := env.AvailabilityZones()
	c.Assert(err, gc.Equals, resultErr)
	c.Assert(zones, gc.IsNil)

	resultErr = nil
	resultZones = make([]amzec2.AvailabilityZoneInfo, 1)
	resultZones[0].Name = "whatever"
	zones, err = env.AvailabilityZones()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(zones, gc.HasLen, 1)
	c.Assert(zones[0].Name(), gc.Equals, "whatever")

	// A successful result is cached, currently for the lifetime
	// of the Environ. This will change if/when we have long-lived
	// Environs to cut down repeated IaaS requests.
	resultErr = fmt.Errorf("failed to get availability zones")
	resultZones[0].Name = "andever"
	zones, err = env.AvailabilityZones()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(zones, gc.HasLen, 1)
	c.Assert(zones[0].Name(), gc.Equals, "whatever")
}

func (t *localServerSuite) TestGetAvailabilityZonesCommon(c *gc.C) {
	var resultZones []amzec2.AvailabilityZoneInfo
	t.PatchValue(ec2.EC2AvailabilityZones, func(e *amzec2.EC2, f *amzec2.Filter) (*amzec2.AvailabilityZonesResp, error) {
		resp := &amzec2.AvailabilityZonesResp{
			Zones: append([]amzec2.AvailabilityZoneInfo{}, resultZones...),
		}
		return resp, nil
	})
	env := t.Prepare(c).(common.ZonedEnviron)
	resultZones = make([]amzec2.AvailabilityZoneInfo, 2)
	resultZones[0].Name = "az1"
	resultZones[1].Name = "az2"
	resultZones[0].State = "available"
	resultZones[1].State = "impaired"
	zones, err := env.AvailabilityZones()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(zones, gc.HasLen, 2)
	c.Assert(zones[0].Name(), gc.Equals, resultZones[0].Name)
	c.Assert(zones[1].Name(), gc.Equals, resultZones[1].Name)
	c.Assert(zones[0].Available(), jc.IsTrue)
	c.Assert(zones[1].Available(), jc.IsFalse)
}

type mockAvailabilityZoneAllocations struct {
	group  []instance.Id // input param
	result []common.AvailabilityZoneInstances
	err    error
}

func (t *mockAvailabilityZoneAllocations) AvailabilityZoneAllocations(
	e common.ZonedEnviron, group []instance.Id,
) ([]common.AvailabilityZoneInstances, error) {
	t.group = group
	return t.result, t.err
}

func (t *localServerSuite) TestStartInstanceDistributionParams(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	mock := mockAvailabilityZoneAllocations{
		result: []common.AvailabilityZoneInstances{{ZoneName: "az1"}},
	}
	t.PatchValue(ec2.AvailabilityZoneAllocations, mock.AvailabilityZoneAllocations)

	// no distribution group specified
	testing.AssertStartInstance(c, env, "1")
	c.Assert(mock.group, gc.HasLen, 0)

	// distribution group specified: ensure it's passed through to AvailabilityZone.
	expectedInstances := []instance.Id{"i-0", "i-1"}
	params := environs.StartInstanceParams{
		DistributionGroup: func() ([]instance.Id, error) {
			return expectedInstances, nil
		},
	}
	_, err = testing.StartInstanceWithParams(env, "1", params, nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(mock.group, gc.DeepEquals, expectedInstances)
}

func (t *localServerSuite) TestStartInstanceDistributionErrors(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	mock := mockAvailabilityZoneAllocations{
		err: fmt.Errorf("AvailabilityZoneAllocations failed"),
	}
	t.PatchValue(ec2.AvailabilityZoneAllocations, mock.AvailabilityZoneAllocations)
	_, _, _, err = testing.StartInstance(env, "1")
	c.Assert(errors.Cause(err), gc.Equals, mock.err)

	mock.err = nil
	dgErr := fmt.Errorf("DistributionGroup failed")
	params := environs.StartInstanceParams{
		DistributionGroup: func() ([]instance.Id, error) {
			return nil, dgErr
		},
	}
	_, err = testing.StartInstanceWithParams(env, "1", params, nil)
	c.Assert(errors.Cause(err), gc.Equals, dgErr)
}

func (t *localServerSuite) TestStartInstanceDistribution(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	// test-available is the only available AZ, so AvailabilityZoneAllocations
	// is guaranteed to return that.
	inst, _ := testing.AssertStartInstance(c, env, "1")
	c.Assert(ec2.InstanceEC2(inst).AvailZone, gc.Equals, "test-available")
}

var azConstrainedErr = &amzec2.Error{
	Code:    "Unsupported",
	Message: "The requested Availability Zone is currently constrained etc.",
}

var azVolumeTypeNotAvailableInZoneErr = &amzec2.Error{
	Code:    "VolumeTypeNotAvailableInZone",
	Message: "blah blah",
}

var azInsufficientInstanceCapacityErr = &amzec2.Error{
	Code: "InsufficientInstanceCapacity",
	Message: "We currently do not have sufficient m1.small capacity in the " +
		"Availability Zone you requested (us-east-1d). Our system will " +
		"be working on provisioning additional capacity. You can currently get m1.small " +
		"capacity by not specifying an Availability Zone in your request or choosing " +
		"us-east-1c, us-east-1a.",
}

var azNoDefaultSubnetErr = &amzec2.Error{
	Code:    "InvalidInput",
	Message: "No default subnet for availability zone: ''us-east-1e''.",
}

func (t *localServerSuite) TestStartInstanceAvailZoneAllConstrained(c *gc.C) {
	t.testStartInstanceAvailZoneAllConstrained(c, azConstrainedErr)
}

func (t *localServerSuite) TestStartInstanceVolumeTypeNotAvailable(c *gc.C) {
	t.testStartInstanceAvailZoneAllConstrained(c, azVolumeTypeNotAvailableInZoneErr)
}

func (t *localServerSuite) TestStartInstanceAvailZoneAllInsufficientInstanceCapacity(c *gc.C) {
	t.testStartInstanceAvailZoneAllConstrained(c, azInsufficientInstanceCapacityErr)
}

func (t *localServerSuite) TestStartInstanceAvailZoneAllNoDefaultSubnet(c *gc.C) {
	t.testStartInstanceAvailZoneAllConstrained(c, azNoDefaultSubnetErr)
}

func (t *localServerSuite) testStartInstanceAvailZoneAllConstrained(c *gc.C, runInstancesError *amzec2.Error) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	mock := mockAvailabilityZoneAllocations{
		result: []common.AvailabilityZoneInstances{
			{ZoneName: "az1"}, {ZoneName: "az2"},
		},
	}
	t.PatchValue(ec2.AvailabilityZoneAllocations, mock.AvailabilityZoneAllocations)

	var azArgs []string
	t.PatchValue(ec2.RunInstances, func(e *amzec2.EC2, ri *amzec2.RunInstances) (*amzec2.RunInstancesResp, error) {
		azArgs = append(azArgs, ri.AvailZone)
		return nil, runInstancesError
	})
	_, _, _, err = testing.StartInstance(env, "1")
	c.Assert(err, gc.ErrorMatches, fmt.Sprintf(
		"cannot run instances: %s \\(%s\\)",
		regexp.QuoteMeta(runInstancesError.Message),
		runInstancesError.Code,
	))
	c.Assert(azArgs, gc.DeepEquals, []string{"az1", "az2"})
}

func (t *localServerSuite) bootstrapAndStartWithParams(c *gc.C, params environs.StartInstanceParams) error {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)
	_, err = testing.StartInstanceWithParams(env, "1", params, nil)
	return err
}

func (t *localServerSuite) TestSpaceConstraintsSpaceNotInPlacementZone(c *gc.C) {
	err := t.bootstrapAndStartWithParams(c, environs.StartInstanceParams{
		Placement:   "zone=test-available",
		Constraints: constraints.MustParse("spaces=aaaaaaaaaa"),
		SubnetsToZones: map[network.Id][]string{
			"subnet-2": []string{"zone2"},
			"subnet-3": []string{"zone3"},
		},
	})

	// Expect an error because zone test-available isn't in SubnetsToZones
	c.Assert(err, gc.ErrorMatches, `unable to resolve constraints: space and/or subnet unavailable in zones \[test-available\]`)
}

func (t *localServerSuite) TestSpaceConstraintsSpaceInPlacementZone(c *gc.C) {
	err := t.bootstrapAndStartWithParams(c, environs.StartInstanceParams{
		Placement:   "zone=test-available",
		Constraints: constraints.MustParse("spaces=aaaaaaaaaa"),
		SubnetsToZones: map[network.Id][]string{
			"subnet-2": []string{"test-available"},
			"subnet-3": []string{"zone3"},
		},
	})

	// Should work - test-available is in SubnetsToZones
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestSpaceConstraintsNoPlacement(c *gc.C) {
	err := t.bootstrapAndStartWithParams(c, environs.StartInstanceParams{
		Constraints: constraints.MustParse("spaces=aaaaaaaaaa"),
		SubnetsToZones: map[network.Id][]string{
			"subnet-2": []string{"test-available"},
			"subnet-3": []string{"zone3"},
		},
	})

	// Shoule work because zone is not specified so we can resolve the constraints
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestSpaceConstraintsNoAvailableSubnets(c *gc.C) {
	err := t.bootstrapAndStartWithParams(c, environs.StartInstanceParams{
		Constraints: constraints.MustParse("spaces=aaaaaaaaaa"),
		SubnetsToZones: map[network.Id][]string{
			"subnet-2": []string{""},
		},
	})

	// We requested a space, but there are no subnets in SubnetsToZones, so we can't resolve
	// the constraints
	c.Assert(err, gc.ErrorMatches, `unable to resolve constraints: space and/or subnet unavailable in zones \[test-available\]`)
}

func (t *localServerSuite) TestStartInstanceAvailZoneOneConstrained(c *gc.C) {
	t.testStartInstanceAvailZoneOneConstrained(c, azConstrainedErr)
}

func (t *localServerSuite) TestStartInstanceAvailZoneOneInsufficientInstanceCapacity(c *gc.C) {
	t.testStartInstanceAvailZoneOneConstrained(c, azInsufficientInstanceCapacityErr)
}

func (t *localServerSuite) TestStartInstanceAvailZoneOneNoDefaultSubnetErr(c *gc.C) {
	t.testStartInstanceAvailZoneOneConstrained(c, azNoDefaultSubnetErr)
}

func (t *localServerSuite) testStartInstanceAvailZoneOneConstrained(c *gc.C, runInstancesError *amzec2.Error) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	mock := mockAvailabilityZoneAllocations{
		result: []common.AvailabilityZoneInstances{
			{ZoneName: "az1"}, {ZoneName: "az2"},
		},
	}
	t.PatchValue(ec2.AvailabilityZoneAllocations, mock.AvailabilityZoneAllocations)

	// The first call to RunInstances fails with an error indicating the AZ
	// is constrained. The second attempt succeeds, and so allocates to az2.
	var azArgs []string
	realRunInstances := *ec2.RunInstances
	t.PatchValue(ec2.RunInstances, func(e *amzec2.EC2, ri *amzec2.RunInstances) (*amzec2.RunInstancesResp, error) {
		azArgs = append(azArgs, ri.AvailZone)
		if len(azArgs) == 1 {
			return nil, runInstancesError
		}
		return realRunInstances(e, ri)
	})
	inst, hwc := testing.AssertStartInstance(c, env, "1")
	c.Assert(azArgs, gc.DeepEquals, []string{"az1", "az2"})
	c.Assert(ec2.InstanceEC2(inst).AvailZone, gc.Equals, "az2")
	c.Check(*hwc.AvailabilityZone, gc.Equals, "az2")
}

func (t *localServerSuite) TestAddresses(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)
	inst, _ := testing.AssertStartInstance(c, env, "1")
	c.Assert(err, jc.ErrorIsNil)
	addrs, err := inst.Addresses()
	c.Assert(err, jc.ErrorIsNil)
	// Expected values use Address type but really contain a regexp for
	// the value rather than a valid ip or hostname.
	expected := []network.Address{{
		Value: "8.0.0.*",
		Type:  network.IPv4Address,
		Scope: network.ScopePublic,
	}, {
		Value: "127.0.0.*",
		Type:  network.IPv4Address,
		Scope: network.ScopeCloudLocal,
	}}
	c.Assert(addrs, gc.HasLen, len(expected))
	for i, addr := range addrs {
		c.Check(addr.Value, gc.Matches, expected[i].Value)
		c.Check(addr.Type, gc.Equals, expected[i].Type)
		c.Check(addr.Scope, gc.Equals, expected[i].Scope)
	}
}

func (t *localServerSuite) TestConstraintsValidatorUnsupported(c *gc.C) {
	env := t.Prepare(c)
	validator, err := env.ConstraintsValidator()
	c.Assert(err, jc.ErrorIsNil)
	cons := constraints.MustParse("arch=amd64 tags=foo")
	unsupported, err := validator.Validate(cons)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(unsupported, gc.DeepEquals, []string{"tags"})
}

func (t *localServerSuite) TestConstraintsValidatorVocab(c *gc.C) {
	env := t.Prepare(c)
	validator, err := env.ConstraintsValidator()
	c.Assert(err, jc.ErrorIsNil)
	cons := constraints.MustParse("arch=ppc64el")
	_, err = validator.Validate(cons)
	c.Assert(err, gc.ErrorMatches, "invalid constraint value: arch=ppc64el\nvalid values are:.*")
	cons = constraints.MustParse("instance-type=foo")
	_, err = validator.Validate(cons)
	c.Assert(err, gc.ErrorMatches, "invalid constraint value: instance-type=foo\nvalid values are:.*")
}

func (t *localServerSuite) TestConstraintsMerge(c *gc.C) {
	env := t.Prepare(c)
	validator, err := env.ConstraintsValidator()
	c.Assert(err, jc.ErrorIsNil)
	consA := constraints.MustParse("arch=amd64 mem=1G cpu-power=10 cpu-cores=2 tags=bar")
	consB := constraints.MustParse("arch=i386 instance-type=m1.small")
	cons, err := validator.Merge(consA, consB)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cons, gc.DeepEquals, constraints.MustParse("arch=i386 instance-type=m1.small tags=bar"))
}

func (t *localServerSuite) TestPrecheckInstanceValidInstanceType(c *gc.C) {
	env := t.Prepare(c)
	cons := constraints.MustParse("instance-type=m1.small root-disk=1G")
	placement := ""
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, cons, placement)
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestPrecheckInstanceInvalidInstanceType(c *gc.C) {
	env := t.Prepare(c)
	cons := constraints.MustParse("instance-type=m1.invalid")
	placement := ""
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, cons, placement)
	c.Assert(err, gc.ErrorMatches, `invalid AWS instance type "m1.invalid" specified`)
}

func (t *localServerSuite) TestPrecheckInstanceUnsupportedArch(c *gc.C) {
	env := t.Prepare(c)
	cons := constraints.MustParse("instance-type=cc1.4xlarge arch=i386")
	placement := ""
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, cons, placement)
	c.Assert(err, gc.ErrorMatches, `invalid AWS instance type "cc1.4xlarge" and arch "i386" specified`)
}

func (t *localServerSuite) TestPrecheckInstanceAvailZone(c *gc.C) {
	env := t.Prepare(c)
	placement := "zone=test-available"
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, constraints.Value{}, placement)
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestPrecheckInstanceAvailZoneUnavailable(c *gc.C) {
	env := t.Prepare(c)
	placement := "zone=test-unavailable"
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, constraints.Value{}, placement)
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestPrecheckInstanceAvailZoneUnknown(c *gc.C) {
	env := t.Prepare(c)
	placement := "zone=test-unknown"
	err := env.PrecheckInstance(coretesting.FakeDefaultSeries, constraints.Value{}, placement)
	c.Assert(err, gc.ErrorMatches, `invalid availability zone "test-unknown"`)
}

func (t *localServerSuite) TestValidateImageMetadata(c *gc.C) {
	env := t.Prepare(c)
	params, err := env.(simplestreams.MetadataValidator).MetadataLookupParams("test")
	c.Assert(err, jc.ErrorIsNil)
	params.Series = coretesting.FakeDefaultSeries
	params.Endpoint = "https://ec2.endpoint.com"
	params.Sources, err = environs.ImageMetadataSources(env)
	c.Assert(err, jc.ErrorIsNil)
	image_ids, _, err := imagemetadata.ValidateImageMetadata(params)
	c.Assert(err, jc.ErrorIsNil)
	sort.Strings(image_ids)
	c.Assert(image_ids, gc.DeepEquals, []string{"ami-00000033", "ami-00000034", "ami-00000035", "ami-00000039"})
}

func (t *localServerSuite) TestGetToolsMetadataSources(c *gc.C) {
	t.PatchValue(&tools.DefaultBaseURL, "")

	env := t.Prepare(c)
	sources, err := tools.GetMetadataSources(env)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(sources, gc.HasLen, 0)
}

func (t *localServerSuite) TestSupportedArchitectures(c *gc.C) {
	env := t.Prepare(c)
	a, err := env.SupportedArchitectures()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(a, jc.SameContents, []string{"amd64", "i386"})
}

func (t *localServerSuite) TestSupportsNetworking(c *gc.C) {
	env := t.Prepare(c)
	_, supported := environs.SupportsNetworking(env)
	c.Assert(supported, jc.IsTrue)
}

func (t *localServerSuite) TestAllocateAddressFailureToFindNetworkInterface(c *gc.C) {
	env := t.prepareEnviron(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	instanceIds, err := env.StateServerInstances()
	c.Assert(err, jc.ErrorIsNil)

	instId := instanceIds[0]
	addr := network.Address{Value: "8.0.0.4"}

	// Invalid instance found
	err = env.AllocateAddress(instId+"foo", "", addr, "foo", "bar")
	c.Assert(err, gc.ErrorMatches, ".*InvalidInstanceID.NotFound.*")

	// No network interface
	err = env.AllocateAddress(instId, "", addr, "foo", "bar")
	c.Assert(errors.Cause(err), gc.ErrorMatches, "unexpected AWS response: network interface not found")
}

func (t *localServerSuite) setUpInstanceWithDefaultVpc(c *gc.C) (environs.NetworkingEnviron, instance.Id) {
	// Simulate a default VPC exists.
	t.srv.ec2srv.AddDefaultVPCAndSubnets()

	env := t.prepareEnviron(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	instanceIds, err := env.StateServerInstances()
	c.Assert(err, jc.ErrorIsNil)
	return env, instanceIds[0]
}

func (t *localServerSuite) TestAllocateAddress(c *gc.C) {
	env, instId := t.setUpInstanceWithDefaultVpc(c)

	addr := network.Address{Value: "8.0.0.4"}
	var actualAddr network.Address
	mockAssign := func(ec2Inst *amzec2.EC2, netId string, addr network.Address) error {
		actualAddr = addr
		return nil
	}
	t.PatchValue(&ec2.AssignPrivateIPAddress, mockAssign)

	err := env.AllocateAddress(instId, "", addr, "foo", "bar")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(actualAddr, gc.Equals, addr)
}

func (t *localServerSuite) TestAllocateAddressIPAddressInUseOrEmpty(c *gc.C) {
	env, instId := t.setUpInstanceWithDefaultVpc(c)

	addr := network.Address{Value: "8.0.0.4"}
	mockAssign := func(ec2Inst *amzec2.EC2, netId string, addr network.Address) error {
		return &amzec2.Error{Code: "InvalidParameterValue"}
	}
	t.PatchValue(&ec2.AssignPrivateIPAddress, mockAssign)

	err := env.AllocateAddress(instId, "", addr, "foo", "bar")
	c.Assert(errors.Cause(err), gc.Equals, environs.ErrIPAddressUnavailable)

	err = env.AllocateAddress(instId, "", network.Address{}, "foo", "bar")
	c.Assert(errors.Cause(err), gc.Equals, environs.ErrIPAddressUnavailable)
}

func (t *localServerSuite) TestAllocateAddressNetworkInterfaceFull(c *gc.C) {
	env, instId := t.setUpInstanceWithDefaultVpc(c)

	addr := network.Address{Value: "8.0.0.4"}
	mockAssign := func(ec2Inst *amzec2.EC2, netId string, addr network.Address) error {
		return &amzec2.Error{Code: "PrivateIpAddressLimitExceeded"}
	}
	t.PatchValue(&ec2.AssignPrivateIPAddress, mockAssign)

	err := env.AllocateAddress(instId, "", addr, "foo", "bar")
	c.Assert(errors.Cause(err), gc.Equals, environs.ErrIPAddressesExhausted)
}

func (t *localServerSuite) TestReleaseAddress(c *gc.C) {
	env, instId := t.setUpInstanceWithDefaultVpc(c)
	addr := network.Address{Value: "8.0.0.4"}
	// Allocate the address first so we can release it
	err := env.AllocateAddress(instId, "", addr, "foo", "bar")
	c.Assert(err, jc.ErrorIsNil)

	err = env.ReleaseAddress(instId, "", addr, "")
	c.Assert(err, jc.ErrorIsNil)

	// Releasing a second time tests that the first call actually released
	// it plus tests the error handling of ReleaseAddress
	err = env.ReleaseAddress(instId, "", addr, "")
	msg := fmt.Sprintf(`failed to release address "8\.0\.0\.4" from instance %q.*`, instId)
	c.Assert(err, gc.ErrorMatches, msg)
}

func (t *localServerSuite) TestReleaseAddressUnknownInstance(c *gc.C) {
	env, _ := t.setUpInstanceWithDefaultVpc(c)

	// We should be able to release an address with an unknown instance id
	// without it being allocated.
	addr := network.Address{Value: "8.0.0.4"}
	err := env.ReleaseAddress(instance.UnknownId, "", addr, "")
	c.Assert(err, jc.ErrorIsNil)
}

func (t *localServerSuite) TestNetworkInterfaces(c *gc.C) {
	env, instId := t.setUpInstanceWithDefaultVpc(c)
	interfaces, err := env.NetworkInterfaces(instId)
	c.Assert(err, jc.ErrorIsNil)

	// The CIDR isn't predictable, but it is in the 10.10.x.0/24 format
	// The subnet ID is in the form "subnet-x", where x matches the same
	// number from the CIDR. The interfaces address is part of the CIDR.
	// For these reasons we check that the CIDR is in the expected format
	// and derive the expected values for ProviderSubnetId and Address.
	c.Assert(interfaces, gc.HasLen, 1)
	cidr := interfaces[0].CIDR
	re := regexp.MustCompile(`10\.10\.(\d+)\.0/24`)
	c.Assert(re.Match([]byte(cidr)), jc.IsTrue)
	index := re.FindStringSubmatch(cidr)[1]
	addr := fmt.Sprintf("10.10.%s.5", index)
	subnetId := network.Id("subnet-" + index)

	expectedInterfaces := []network.InterfaceInfo{{
		DeviceIndex:      0,
		MACAddress:       "20:01:60:cb:27:37",
		CIDR:             cidr,
		ProviderId:       "eni-0",
		ProviderSubnetId: subnetId,
		VLANTag:          0,
		InterfaceName:    "unsupported0",
		Disabled:         false,
		NoAutoStart:      false,
		ConfigType:       network.ConfigDHCP,
		Address:          network.NewScopedAddress(addr, network.ScopeCloudLocal),
	}}
	c.Assert(interfaces, jc.DeepEquals, expectedInterfaces)
}

func validateSubnets(c *gc.C, subnets []network.SubnetInfo) {
	// These are defined in the test server for the testing default
	// VPC.
	defaultSubnets := []network.SubnetInfo{{
		CIDR:              "10.10.0.0/24",
		ProviderId:        "subnet-0",
		VLANTag:           0,
		AllocatableIPLow:  net.ParseIP("10.10.0.4").To4(),
		AllocatableIPHigh: net.ParseIP("10.10.0.254").To4(),
		AvailabilityZones: []string{"test-available"},
	}, {
		CIDR:              "10.10.1.0/24",
		ProviderId:        "subnet-1",
		VLANTag:           0,
		AllocatableIPLow:  net.ParseIP("10.10.1.4").To4(),
		AllocatableIPHigh: net.ParseIP("10.10.1.254").To4(),
		AvailabilityZones: []string{"test-impaired"},
	}, {
		CIDR:              "10.10.2.0/24",
		ProviderId:        "subnet-2",
		VLANTag:           0,
		AllocatableIPLow:  net.ParseIP("10.10.2.4").To4(),
		AllocatableIPHigh: net.ParseIP("10.10.2.254").To4(),
		AvailabilityZones: []string{"test-unavailable"},
	}}

	for _, subnet := range subnets {
		// We can find the expected data by looking at the CIDR.
		// subnets isn't in a predictable order due to the use of maps.
		re := regexp.MustCompile(`10\.10\.(\d+)\.0/24`)
		c.Assert(re.Match([]byte(subnet.CIDR)), jc.IsTrue)
		index, err := strconv.Atoi(re.FindStringSubmatch(subnet.CIDR)[1])
		c.Assert(err, jc.ErrorIsNil)
		// Don't know which AZ the subnet will end up in.
		defaultSubnets[index].AvailabilityZones = subnet.AvailabilityZones
		c.Assert(subnet, jc.DeepEquals, defaultSubnets[index])
	}
}

func (t *localServerSuite) TestSubnets(c *gc.C) {
	env, _ := t.setUpInstanceWithDefaultVpc(c)

	subnets, err := env.Subnets(instance.UnknownId, []network.Id{"subnet-0"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(subnets, gc.HasLen, 1)
	validateSubnets(c, subnets)

	subnets, err = env.Subnets(instance.UnknownId, nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(subnets, gc.HasLen, 3)
	validateSubnets(c, subnets)
}

func (t *localServerSuite) TestSubnetsInstIdNotSupported(c *gc.C) {
	env, _ := t.setUpInstanceWithDefaultVpc(c)

	_, err := env.Subnets("foo", []network.Id{})
	c.Assert(err, gc.ErrorMatches, "instId not supported")
}

func (t *localServerSuite) TestSubnetsMissingSubnet(c *gc.C) {
	env, _ := t.setUpInstanceWithDefaultVpc(c)

	_, err := env.Subnets("", []network.Id{"subnet-0", "Missing"})
	c.Assert(err, gc.ErrorMatches, `failed to find the following subnet ids: \[Missing\]`)
}

func (t *localServerSuite) TestSupportsAddressAllocationTrue(c *gc.C) {
	t.srv.ec2srv.AddDefaultVPCAndSubnets()
	env := t.prepareEnviron(c)
	result, err := env.SupportsAddressAllocation("")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.IsTrue)
}

func (t *localServerSuite) TestSupportsAddressAllocationWithNoFeatureFlag(c *gc.C) {
	t.SetFeatureFlags() // clear the flags.
	env := t.prepareEnviron(c)
	result, err := env.SupportsAddressAllocation("")
	c.Assert(err, gc.ErrorMatches, "address allocation not supported")
	c.Assert(err, jc.Satisfies, errors.IsNotSupported)
	c.Assert(result, jc.IsFalse)
}

func (t *localServerSuite) TestAllocateAddressWithNoFeatureFlag(c *gc.C) {
	t.SetFeatureFlags() // clear the flags.
	env := t.prepareEnviron(c)
	err := env.AllocateAddress("i-foo", "net1", network.NewAddresses("1.2.3.4")[0], "foo", "bar")
	c.Assert(err, gc.ErrorMatches, "address allocation not supported")
	c.Assert(err, jc.Satisfies, errors.IsNotSupported)
}

func (t *localServerSuite) TestReleaseAddressWithNoFeatureFlag(c *gc.C) {
	t.SetFeatureFlags() // clear the flags.
	env := t.prepareEnviron(c)
	err := env.ReleaseAddress("i-foo", "net1", network.NewAddress("1.2.3.4"), "")
	c.Assert(err, gc.ErrorMatches, "address allocation not supported")
	c.Assert(err, jc.Satisfies, errors.IsNotSupported)
}

func (t *localServerSuite) TestSupportsAddressAllocationCaches(c *gc.C) {
	t.srv.ec2srv.SetAccountAttributes(map[string][]string{
		"default-vpc": {"none"},
	})
	env := t.prepareEnviron(c)
	result, err := env.SupportsAddressAllocation("")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.IsFalse)

	// this value won't change normally, the change here is to
	// ensure that subsequent calls use the cached value
	t.srv.ec2srv.SetAccountAttributes(map[string][]string{
		"default-vpc": {"vpc-xxxxxxx"},
	})
	result, err = env.SupportsAddressAllocation("")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.IsFalse)
}

func (t *localServerSuite) TestSupportsAddressAllocationFalse(c *gc.C) {
	t.srv.ec2srv.SetAccountAttributes(map[string][]string{
		"default-vpc": {"none"},
	})
	env := t.prepareEnviron(c)
	result, err := env.SupportsAddressAllocation("")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.IsFalse)
}

func (t *localServerSuite) TestInstanceTags(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	instances, err := env.AllInstances()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(instances, gc.HasLen, 1)

	ec2Inst := ec2.InstanceEC2(instances[0])
	c.Assert(ec2Inst.Tags, jc.SameContents, []amzec2.Tag{
		{"Name", "juju-sample-machine-0"},
		{"juju-env-uuid", coretesting.EnvironmentTag.Id()},
		{"juju-is-state", "true"},
	})
}

func (t *localServerSuite) TestRootDiskTags(c *gc.C) {
	env := t.Prepare(c)
	err := bootstrap.Bootstrap(envtesting.BootstrapContext(c), env, bootstrap.BootstrapParams{})
	c.Assert(err, jc.ErrorIsNil)

	instances, err := env.AllInstances()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(instances, gc.HasLen, 1)

	ec2conn := ec2.EnvironEC2(env)
	resp, err := ec2conn.Volumes(nil, nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(resp.Volumes, gc.Not(gc.HasLen), 0)

	var found *amzec2.Volume
	for _, vol := range resp.Volumes {
		if len(vol.Tags) != 0 {
			found = &vol
			break
		}
	}
	c.Assert(found, gc.NotNil)
	c.Assert(found.Tags, jc.SameContents, []amzec2.Tag{
		{"Name", "juju-sample-machine-0-root"},
		{"juju-env-uuid", coretesting.EnvironmentTag.Id()},
	})
}

// localNonUSEastSuite is similar to localServerSuite but the S3 mock server
// behaves as if it is not in the us-east region.
type localNonUSEastSuite struct {
	coretesting.BaseSuite
	restoreEC2Patching func()
	srv                localServer
	env                environs.Environ
}

func (t *localNonUSEastSuite) SetUpSuite(c *gc.C) {
	t.BaseSuite.SetUpSuite(c)
	t.restoreEC2Patching = patchEC2ForTesting()
}

func (t *localNonUSEastSuite) TearDownSuite(c *gc.C) {
	t.restoreEC2Patching()
	t.BaseSuite.TearDownSuite(c)
}

func (t *localNonUSEastSuite) SetUpTest(c *gc.C) {
	t.BaseSuite.SetUpTest(c)
	t.srv.config = &s3test.Config{
		Send409Conflict: true,
	}
	t.srv.startServer(c)

	cfg, err := config.New(config.NoDefaults, localConfigAttrs)
	c.Assert(err, jc.ErrorIsNil)
	env, err := environs.Prepare(cfg, envtesting.BootstrapContext(c), configstore.NewMem())
	c.Assert(err, jc.ErrorIsNil)
	t.env = env
}

func (t *localNonUSEastSuite) TearDownTest(c *gc.C) {
	t.srv.stopServer(c)
	t.BaseSuite.TearDownTest(c)
}

func patchEC2ForTesting() func() {
	ec2.UseTestImageData(ec2.TestImagesData)
	ec2.UseTestInstanceTypeData(ec2.TestInstanceTypeCosts)
	ec2.UseTestRegionData(ec2.TestRegions)
	restoreTimeouts := envtesting.PatchAttemptStrategies(ec2.ShortAttempt, ec2.StorageAttempt)
	restoreFinishBootstrap := envtesting.DisableFinishBootstrap()
	return func() {
		restoreFinishBootstrap()
		restoreTimeouts()
		ec2.UseTestImageData(nil)
		ec2.UseTestInstanceTypeData(nil)
		ec2.UseTestRegionData(nil)
	}
}

// If match is true, CheckScripts checks that at least one script started
// by the cloudinit data matches the given regexp pattern, otherwise it
// checks that no script matches.  It's exported so it can be used by tests
// defined in ec2_test.
func CheckScripts(c *gc.C, userDataMap map[interface{}]interface{}, pattern string, match bool) {
	scripts0 := userDataMap["runcmd"]
	if scripts0 == nil {
		c.Errorf("cloudinit has no entry for runcmd")
		return
	}
	scripts := scripts0.([]interface{})
	re := regexp.MustCompile(pattern)
	found := false
	for _, s0 := range scripts {
		s := s0.(string)
		if re.MatchString(s) {
			found = true
		}
	}
	switch {
	case match && !found:
		c.Errorf("script %q not found in %q", pattern, scripts)
	case !match && found:
		c.Errorf("script %q found but not expected in %q", pattern, scripts)
	}
}

// CheckPackage checks that the cloudinit will or won't install the given
// package, depending on the value of match.  It's exported so it can be
// used by tests defined outside the ec2 package.
func CheckPackage(c *gc.C, userDataMap map[interface{}]interface{}, pkg string, match bool) {
	pkgs0 := userDataMap["packages"]
	if pkgs0 == nil {
		if match {
			c.Errorf("cloudinit has no entry for packages")
		}
		return
	}

	pkgs := pkgs0.([]interface{})

	found := false
	for _, p0 := range pkgs {
		p := p0.(string)
		// p might be a space separate list of packages eg 'foo bar qed' so split them up
		manyPkgs := set.NewStrings(strings.Split(p, " ")...)
		hasPkg := manyPkgs.Contains(pkg)
		if p == pkg || hasPkg {
			found = true
			break
		}
	}
	switch {
	case match && !found:
		c.Errorf("package %q not found in %v", pkg, pkgs)
	case !match && found:
		c.Errorf("%q found but not expected in %v", pkg, pkgs)
	}
}
