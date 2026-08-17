using System.Diagnostics;
using System.Windows;
using System.Windows.Threading;
using DelayForge.Engine;
using DelayForge.Models;

namespace DelayForge;

public partial class MainWindow : Window
{
    private readonly DivertEngine _engine;
    private readonly DispatcherTimer _statsTimer;
    private bool _isRunning;

    public MainWindow()
    {
        InitializeComponent();

        var stats = new Stats();
        var config = BuildConfig();
        _engine = new DivertEngine(stats, config);

        // Stats refresh timer: 500ms
        _statsTimer = new DispatcherTimer
        {
            Interval = TimeSpan.FromMilliseconds(500)
        };
        _statsTimer.Tick += StatsTimer_Tick;

        UpdateFilterPreview();
        UpdateButtonState();
    }

    private DamageConfig BuildConfig()
    {
        return new DamageConfig
        {
            LatencyMs = GetSliderInt(SliderLatency, TxtLatency),
            JitterMs = GetSliderInt(SliderJitter, TxtJitter),
            PacketLossPercent = GetSliderDouble(SliderLoss, TxtLoss),
            DuplicatePercent = GetSliderDouble(SliderDuplicate, TxtDuplicate),
            ReorderPercent = GetSliderDouble(SliderReorder, TxtReorder),
            TamperPercent = GetSliderDouble(SliderTamper, TxtTamper),
            ThrottleKbps = GetSliderInt(SliderThrottle, TxtThrottle),
            Direction = CmbDirection.SelectedIndex switch
            {
                1 => FilterDirection.Outbound,
                2 => FilterDirection.Inbound,
                _ => FilterDirection.Both
            },
            Protocol = CmbProtocol.SelectedIndex switch
            {
                1 => FilterProtocol.Tcp,
                2 => FilterProtocol.Udp,
                3 => FilterProtocol.Icmp,
                _ => FilterProtocol.Any
            },
            ProcessFilter = TxtProcessFilter.Text.Trim(),
            IpFilter = TxtIpFilter.Text.Trim(),
            PortFilter = TxtPortFilter.Text.Trim()
        };
    }

    private int GetSliderInt(System.Windows.Controls.Slider slider, System.Windows.Controls.TextBox txt)
    {
        if (int.TryParse(txt.Text, out int val))
            return Math.Clamp(val, (int)slider.Minimum, (int)slider.Maximum);
        return (int)slider.Value;
    }

    private double GetSliderDouble(System.Windows.Controls.Slider slider, System.Windows.Controls.TextBox txt)
    {
        if (double.TryParse(txt.Text, out double val))
            return Math.Clamp(val, slider.Minimum, slider.Maximum);
        return slider.Value;
    }

    // --- Toggle Start/Stop ---
    private void BtnToggle_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            if (_isRunning)
            {
                _engine.Stop();
                _statsTimer.Stop();
                _isRunning = false;
                Debug.WriteLine("[DelayForge] Stopped by user.");
            }
            else
            {
                // Update config before starting
                var config = BuildConfig();
                _engine.UpdateConfig(config);
                _engine.Start();
                _statsTimer.Start();
                _isRunning = true;
                Debug.WriteLine("[DelayForge] Started by user.");
            }
        }
        catch (InvalidOperationException ex)
        {
            MessageBox.Show(ex.Message, "DelayForge Error", MessageBoxButton.OK, MessageBoxImage.Error);
        }
        catch (Exception ex)
        {
            MessageBox.Show($"Unexpected error: {ex.Message}", "DelayForge Error", MessageBoxButton.OK, MessageBoxImage.Error);
        }

        UpdateButtonState();
    }

    private void UpdateButtonState()
    {
        if (_isRunning)
        {
            BtnLabel.Text = "⏹  Stop";
            BtnLabel.Foreground = System.Windows.Media.Brushes.White;
            BtnToggle.Background = new System.Windows.Media.SolidColorBrush(
                System.Windows.Media.Color.FromRgb(0xDC, 0x26, 0x26)); // Red
        }
        else
        {
            BtnLabel.Text = "▶  Start";
            BtnLabel.Foreground = System.Windows.Media.Brushes.White;
            BtnToggle.Background = new System.Windows.Media.SolidColorBrush(
                System.Windows.Media.Color.FromRgb(0x16, 0xA3, 0x4A)); // Green
        }
    }

    // --- Slider <-> TextBox sync ---
    private void SliderLatency_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtLatency != null) TxtLatency.Text = ((int)SliderLatency.Value).ToString();
        UpdateFilterPreview();
    }
    private void TxtLatency_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderLatency, TxtLatency);
    }

    private void SliderJitter_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtJitter != null) TxtJitter.Text = ((int)SliderJitter.Value).ToString();
    }
    private void TxtJitter_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderJitter, TxtJitter);
    }

    private void SliderLoss_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtLoss != null) TxtLoss.Text = ((int)SliderLoss.Value).ToString();
    }
    private void TxtLoss_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderLoss, TxtLoss);
    }

    private void SliderDuplicate_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtDuplicate != null) TxtDuplicate.Text = ((int)SliderDuplicate.Value).ToString();
    }
    private void TxtDuplicate_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderDuplicate, TxtDuplicate);
    }

    private void SliderReorder_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtReorder != null) TxtReorder.Text = ((int)SliderReorder.Value).ToString();
    }
    private void TxtReorder_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderReorder, TxtReorder);
    }

    private void SliderTamper_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtTamper != null) TxtTamper.Text = ((int)SliderTamper.Value).ToString();
    }
    private void TxtTamper_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderTamper, TxtTamper);
    }

    private void SliderThrottle_ValueChanged(object sender, RoutedPropertyChangedEventArgs<double> e)
    {
        if (TxtThrottle != null) TxtThrottle.Text = ((int)SliderThrottle.Value).ToString();
    }
    private void TxtThrottle_LostFocus(object sender, System.Windows.RoutedEventArgs e)
    {
        SyncSliderFromText(SliderThrottle, TxtThrottle);
    }

    private static void SyncSliderFromText(System.Windows.Controls.Slider slider, System.Windows.Controls.TextBox txt)
    {
        if (int.TryParse(txt.Text, out int val))
        {
            slider.Value = Math.Clamp(val, (int)slider.Minimum, (int)slider.Maximum);
        }
        txt.Text = ((int)slider.Value).ToString();
    }

    // --- Live Stats ---
    private void StatsTimer_Tick(object? sender, EventArgs e)
    {
        var s = _engine.Statistics;
        StatProcessed.Text = s.TotalProcessed.ToString("N0");
        StatBytes.Text = FormatBytes(s.TotalBytes);
        StatDelayed.Text = s.TotalDelayed.ToString("N0");
        StatDropped.Text = s.TotalDropped.ToString("N0");
        StatDuplicated.Text = s.TotalDuplicated.ToString("N0");
        StatReordered.Text = s.TotalReordered.ToString("N0");
        StatTampered.Text = s.TotalTampered.ToString("N0");
        StatQueueDepth.Text = $"Delay:{_engine.DelayQueueDepth}  Reorder:{_engine.ReorderQueueDepth}  Throttle:{_engine.ThrottleQueueDepth}";

        // Update config on the fly if running
        if (_isRunning)
        {
            _engine.UpdateConfig(BuildConfig());
        }
    }

    private static string FormatBytes(long bytes)
    {
        if (bytes < 1024) return $"{bytes} B";
        if (bytes < 1024 * 1024) return $"{bytes / 1024.0:F1} KB";
        if (bytes < 1024 * 1024 * 1024) return $"{bytes / (1024.0 * 1024):F1} MB";
        return $"{bytes / (1024.0 * 1024 * 1024):F2} GB";
    }

    // --- Filter Preview ---
    private void UpdateFilterPreview()
    {
        try
        {
            var config = BuildConfig();
            TxtFilterPreview.Text = FilterCompiler.Compile(config);
        }
        catch { }
    }

    protected override void OnClosed(EventArgs e)
    {
        _statsTimer.Stop();
        _engine?.Dispose();
        base.OnClosed(e);
    }
}
