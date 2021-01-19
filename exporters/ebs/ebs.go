package ebs

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/sirupsen/logrus"
	"github.com/thunderbottom/ebs-exporter/pkg/config"
	"github.com/thunderbottom/ebs-exporter/pkg/exporter"
	"golang.org/x/sync/errgroup"
)

const (
	namespace = "ebs"
)

// EBSExporter is a struct that holds an instance
// of EC2 client and the job details required to
// scrape EBS metrics
type EBSExporter struct {
	client     *ec2.EC2
	cloudwatch *cloudwatch.CloudWatch
	filters    []*ec2.Filter
	job        *config.Job
	logger     *logrus.Logger
	metrics    *metrics.Set
}

// New returns a new instance of EBSExporter
func New(j *config.Job, log *logrus.Logger, m *metrics.Set, rc *aws.Config, sess *session.Session) *EBSExporter {
	// create instances of ec2 and cloudwatch clients
	var (
		client *ec2.EC2
		cw     *cloudwatch.CloudWatch
	)
	// RoleARN config overrides Access Key and Secret
	if rc != nil {
		client = ec2.New(sess, rc)
		cw = cloudwatch.New(sess, rc)
	} else {
		client = ec2.New(sess)
		cw = cloudwatch.New(sess)
	}

	filters := make([]*ec2.Filter, 0, len(j.Filters))
	for _, tag := range j.Filters {
		if tag.Name != "" || tag.Value != "" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(tag.Name),
				Values: []*string{aws.String(tag.Value)},
			})
		}
	}

	log.Debugf("%s: setting up a new EBS exporter", j.Name)
	return &EBSExporter{
		client:     client,
		cloudwatch: cw,
		filters:    filters,
		job:        j,
		logger:     log,
		metrics:    m,
	}
}

// Collect calls methods to collect metrics from AWS
func (e *EBSExporter) Collect() error {
	var g errgroup.Group
	g.Go(e.getSnapshotMetrics)
	g.Go(e.getVolumeStatusMetrics)
	g.Go(e.getVolumeUsageMetrics)

	// Return if any of the errgroup
	// goroutines returns an error
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// getSnapshotMetrics scrapes EBS volume snapshot metrics from AWS
func (e *EBSExporter) getSnapshotMetrics() error {
	input := &ec2.DescribeSnapshotsInput{}
	// Check whether there are filters defined in the config
	if len(e.filters) != 0 {
		input.Filters = e.filters
	}
	// Fetch only private snapshots
	input.OwnerIds = []*string{aws.String("self")}

	snapshots, err := e.client.DescribeSnapshots(input)
	if err != nil {
		e.logger.Errorf("an error occurred while retrieving snapshot data: %s", err)
		return err
	}

	e.logger.Debugf("%s: got %d Volume Snapshots", e.job.Name, len(snapshots.Snapshots))
	for _, s := range snapshots.Snapshots {
		// Default labels to attach to all metrics
		labels := fmt.Sprintf(`job="%s",region="%s",snapshot_id="%s",vol_id="%s",progress="%s",state="%s"`,
			e.job.Name, e.job.AWS.Region, *s.SnapshotId, *s.VolumeId, *s.Progress, *s.State)

		// Check whether the snapshot has any tags
		// that we want to export
		for _, et := range e.job.Tags {
			for _, t := range s.Tags {
				if *t.Key == et.Tag {
					// Ensure that the tags are correct by replacing
					// unsupported characters with underscore
					labels = labels + fmt.Sprintf(`,%s="%s"`, exporter.FormatTag(et.ExportedTag), *t.Value)
				}
			}
		}

		// Total number of EBS snapshots
		snapTotal := fmt.Sprintf(`%s_snapshots_total{job="%s",region="%s",state="%s"}`,
			namespace, e.job.Name, e.job.AWS.Region, *s.State)
		e.metrics.GetOrCreateCounter(snapTotal).Add(1)

		// Size of the volume associated with the EBS snapshot
		volSize := fmt.Sprintf(`%s_snapshots_volume_size{%s}`, namespace, labels)
		vsize := float64(*s.VolumeSize)
		e.metrics.GetOrCreateGauge(volSize, func() float64 {
			return vsize
		})

		// Start Time of the EBS Snapshot (UNIX Time)
		snapStartTime := fmt.Sprintf(`%s_snapshots_start_time{%s}`, namespace, labels)
		sstart := float64(s.StartTime.Unix())
		e.metrics.GetOrCreateGauge(snapStartTime, func() float64 {
			return sstart
		})
	}

	return nil
}

// getVolumeStatusMetrics scrapes EBS volume status metrics from AWS
func (e *EBSExporter) getVolumeStatusMetrics() error {
	input := &ec2.DescribeVolumeStatusInput{}
	if len(e.filters) != 0 {
		input.Filters = e.filters
	}

	volumes, err := e.client.DescribeVolumeStatus(input)
	if err != nil {
		e.logger.Errorf("an error occurred while retrieving volume status data: %s", err)
		return err
	}

	e.logger.Debugf("%s: Got %d Volume Statuses", e.job.Name, len(volumes.VolumeStatuses))
	for _, v := range volumes.VolumeStatuses {
		// Default labels to attach to all metrics
		labels := fmt.Sprintf(`job="%s",region="%s",vol_id="%s"`,
			e.job.Name, e.job.AWS.Region, *v.VolumeId)

		// Convert volume status to numbers
		// ok => 0, warning => 1, impaired => 2, insufficient-data => 3
		var status int
		switch *v.VolumeStatus.Status {
		case "ok":
			status = 0
		case "warning":
			status = 1
		case "impaired":
			status = 2
		case "insufficient-data":
			status = 3
		}

		// Total number of volumes by status
		statTotal := fmt.Sprintf(`%s_volume_total{job="%s",region="%s",status="%s"}`,
			namespace, e.job.Name, e.job.AWS.Region, *v.VolumeStatus.Status)
		e.metrics.GetOrCreateCounter(statTotal).Add(1)

		// EBS volume status
		volStatus := fmt.Sprintf(`%s_volume_status{%s}`, namespace, labels)
		e.metrics.GetOrCreateGauge(volStatus, func() float64 {
			return float64(status)
		})
	}

	return nil
}

// getVolumeUsageMetrics scrapes EBS volume usage metrics from AWS
func (e *EBSExporter) getVolumeUsageMetrics() error {
	input := &ec2.DescribeVolumesInput{}
	if len(e.filters) != 0 {
		input.Filters = e.filters
	}

	volumes, err := e.client.DescribeVolumes(input)
	if err != nil {
		e.logger.Errorf("An error occurred while retrieving volume usage data: %s", err)
		return err
	}

	e.logger.Debugf("%s: Got %d Volumes", e.job.Name, len(volumes.Volumes))
	for _, v := range volumes.Volumes {
		// Default labels to attach to all metrics
		labels := fmt.Sprintf(`job="%s",region="%s",vol_id="%s"`,
			e.job.Name, e.job.AWS.Region, *v.VolumeId)

		// Check whether the volume contains any tags
		// that we want to export
		for _, et := range e.job.Tags {
			for _, t := range v.Tags {
				if *t.Key == et.Tag {
					// Ensure that the tags are correct by replacing
					// unsupported characters with underscore
					labels = labels + fmt.Sprintf(`,%s="%s"`, exporter.FormatTag(et.ExportedTag), *t.Value)
				}
			}
		}

		// Total number of volumes by volume-type and availability zone
		typeTotal := fmt.Sprintf(`%s_volume_type_total{job="%s",region="%s",type="%s",availability_zone="%s"}`,
			namespace, e.job.Name, e.job.AWS.Region, *v.VolumeType, *v.AvailabilityZone)
		e.metrics.GetOrCreateCounter(typeTotal).Add(1)

		// Total number of volumes by usage and availability,
		// and volume availability zone
		usageTotal := fmt.Sprintf(`%s_volume_usage_status_total{job="%s",region="%s",status="%s",availability_zone="%s"}`,
			namespace, e.job.Name, e.job.AWS.Region, *v.State, *v.AvailabilityZone)
		e.metrics.GetOrCreateCounter(usageTotal).Add(1)

		// Get EBS BurstBalance metrics
		balance, err := e.getIOPSBalance(*v.VolumeId)
		if err != nil {
			e.logger.Errorf("an error occurred while retrieving volume IOPS data: %s", err)
			return err
		}

		volIOPS := fmt.Sprintf(`%s_volume_iops_credit{job="%s",region="%s",vol_id="%s"}`,
			namespace, e.job.Name, e.job.AWS.Region, *v.VolumeId)
		e.metrics.GetOrCreateGauge(volIOPS, func() float64 {
			return balance
		})
	}

	return nil
}

// getIOPSBalance gets last 5-minute average IOPS BurstBalance
// for an EBS volume using AWS Cloudwatch
func (e *EBSExporter) getIOPSBalance(volumeID string) (float64, error) {
	input := &cloudwatch.GetMetricStatisticsInput{
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  aws.String("VolumeId"),
				Value: aws.String(volumeID),
			},
		},
		MetricName: aws.String("BurstBalance"),
		Namespace:  aws.String("AWS/EBS"),
		// Get IOPS average for the last 5 minutes
		// Setting Period to 5 minutes gives a single
		// average value for the entire duration
		Statistics: []*string{aws.String("Average")},
		Period:     aws.Int64(5 * 60),
		StartTime:  aws.Time(time.Now().Add(time.Duration(-5) * time.Minute)),
		EndTime:    aws.Time(time.Now()),
	}

	metrics, err := e.cloudwatch.GetMetricStatistics(input)
	if err != nil {
		return 0, err
	}

	// Some volumes do not have any IOPS value
	if metrics != nil && metrics.Datapoints != nil && len(metrics.Datapoints) >= 1 {
		var avgIOPS, totalIOPS float64
		for _, datapoint := range metrics.Datapoints {
			totalIOPS += *datapoint.Average
		}
		avgIOPS = totalIOPS / float64(len(metrics.Datapoints))
		return avgIOPS, nil
	}

	e.logger.Debugf("%s: volume %s has no IOPS BurstBalance", e.job.Name, volumeID)
	return 0, nil
}
