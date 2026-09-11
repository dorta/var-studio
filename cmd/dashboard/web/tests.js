const $ = (id) => document.getElementById(id);
const state = {
  capabilities: new Map(),
  guides: [],
  selected: null,
  filter: "all",
  category: "all",
  query: "",
};
const esc = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#039;",
      })[c],
  );
const step = (title, text, command = "") => ({
  title,
  text,
  command,
});
const guide = (
  id,
  icon,
  title,
  category,
  capability,
  mode,
  objective,
  prerequisites,
  steps,
  expected,
  note = "",
) => ({
  id,
  icon,
  title,
  category,
  capability,
  mode,
  objective,
  prerequisites,
  steps,
  expected,
  note,
});
const guides = [
  guide(
    "build",
    "BSP",
    "Build provenance",
    "System",
    null,
    "Observe",
    "Confirm the exact distro, kernel, BSP release, a" +
      "nd layer revisions used by this image.",
    "No external equipment.",
    [
      step(
        "Open the BSP inventory",
        "Review distro, release, kernel, layer status, an" +
          "d commit revisions.",
        "cat /etc/buildinfo",
      ),
      step(
        "Record the running kernel",
        "Compare the live kernel with the build provenanc" + "e.",
        "uname -a",
      ),
    ],
    "The running kernel and build metadata are intern" +
      "ally consistent. Modified layers are explicitly " +
      "identified.",
  ),
  guide(
    "kernel",
    "LOG",
    "Kernel and Device Tree",
    "System",
    null,
    "Observe",
    "Find driver probe failures and confirm the Devic" +
      "e Tree selected at boot.",
    "No external equipment.",
    [
      step(
        "Inspect important messages",
        "Start with errors and warnings; avoid treating e" +
          "very historical warning as a current failure.",
        "dmesg --level=emerg,alert,crit,err,warn",
      ),
      step(
        "Read compatible strings",
        "Use these strings to identify the SoM, carrier, " + "and SoC.",
        "tr '\\0' '\\n' < /proc/device-tree/compatible",
      ),
      step(
        "Check disabled nodes",
        "Review the live flattened tree when dtc is insta" + "lled.",
        "dtc -I fs /sys/firmware/devicetree/base 2>/dev/n" +
          "ull | grep -B 4 'status = \"disabled\"'",
      ),
    ],
    "Required drivers probe successfully and the comp" +
      "atible strings match the physical platform.",
  ),
  guide(
    "cpu",
    "CPU",
    "CPU and load",
    "Compute",
    null,
    "Observe",
    "Validate CPU topology, frequency exposure, sched" +
      "uler load, and a short compute workload.",
    "Run the workload only when temporary CPU load is" + " acceptable.",
    [
      step(
        "Inspect topology",
        "Confirm the expected cores are online.",
        "lscpu 2>/dev/null || cat /proc/cpuinfo",
      ),
      step(
        "Observe load",
        "Watch per-core activity without changing state.",
        "top -b -n 1",
      ),
      step(
        "Optional short workload",
        "Use the packaged release test when it is install" + "ed.",
        "var-test test_cpu --help",
      ),
    ],
    "All expected cores are online, load settles afte" +
      "r the workload, and no thermal or kernel error i" +
      "s emitted.",
  ),
  guide(
    "thermal",
    "°C",
    "Thermal sensors",
    "Compute",
    null,
    "Observe",
    "Confirm thermal zones return plausible values an" +
      "d remain below platform limits.",
    "Avoid stressing an uncooled board.",
    [
      step(
        "Read thermal zones",
        "Print each zone name and current temperature.",
        "for z in /sys/class/thermal/thermal_zone*; do pr" +
          'intf \'%s: \' "$(cat "$z/type")"; awk \'{printf "%.' +
          '1f\xB0C\\n", $1/1000}\' "$z/temp"; done',
      ),
      step(
        "Watch trends",
        "Use VAR-Studio Dashboard to correlate CPU, memory" +
          ", GPU, and thermals.",
        "",
      ),
    ],
    "Temperatures are plausible, stable for the workl" +
      "oad, and no thermal throttling warning appears.",
  ),
  guide(
    "ethernet",
    "NET",
    "Ethernet",
    "Connectivity",
    null,
    "Interactive",
    "Validate PHY link, addressing, counters, and end" + "-to-end traffic.",
    "Connected peer or LAN; iperf3 server for through" + "put measurement.",
    [
      step(
        "Inspect links and counters",
        "Check carrier, address, drops, and errors.",
        "ip -s address",
      ),
      step(
        "Test reachability",
        "Replace the address with your known peer.",
        "ping -c 4 <peer-ip>",
      ),
      step(
        "Optional throughput",
        "Run against a controlled iperf3 server.",
        "iperf3 -c <server-ip> -t 10",
      ),
    ],
    "Link stays up, packet loss is zero on the local " +
      "link, error counters do not grow, and throughput" +
      " meets the interface target.",
  ),
  guide(
    "wifi",
    "WIFI",
    "Wi-Fi",
    "Connectivity",
    "wifi",
    "Interactive",
    "Validate discovery, association, addressing, and" + " network traffic.",
    "Known test access point and credentials. Command" +
      "s depend on the image network manager.",
    [
      step(
        "Inspect the interface",
        "Confirm driver and link state.",
        "ip -s link show wlan0",
      ),
      step("List radio information", "Use iw when available.", "iw dev"),
      step(
        "Verify connectivity",
        "Test a known gateway after association.",
        "ping -c 4 <gateway-ip>",
      ),
    ],
    "The interface associates, obtains an address, an" +
      "d transfers traffic without increasing error cou" +
      "nters.",
  ),
  guide(
    "bluetooth",
    "BT",
    "Bluetooth",
    "Connectivity",
    "bluetooth",
    "Interactive",
    "Confirm controller enumeration, discovery, pairi" +
      "ng, and an optional file or audio transfer.",
    "A known Bluetooth peer. Pairing changes controll" + "er state.",
    [
      step(
        "Inspect controllers",
        "Confirm an HCI controller is present.",
        "bluetoothctl list",
      ),
      step(
        "Interactive discovery",
        "Run only in an RF-safe test environment.",
        "bluetoothctl scan on",
      ),
      step(
        "Collect evidence",
        "Record controller and peer information.",
        "bluetoothctl show",
      ),
    ],
    "A controller is powered, the known peer is disco" +
      "vered, and the selected transfer completes.",
  ),
  guide(
    "can",
    "CAN",
    "CAN bus",
    "Connectivity",
    "can",
    "Interactive",
    "Validate CAN pinmux, controller state, transmit," + " and receive paths.",
    "Second CAN node or an approved loopback harness;" +
      " correct transceiver and bitrate.",
    [
      step(
        "Configure the interface",
        "Choose the bitrate required by the test network.",
        "ip link set can0 down; ip link set can0 type can" +
          " bitrate 500000; ip link set can0 up",
      ),
      step(
        "Receive on one endpoint",
        "Run candump before transmitting.",
        "candump can0",
      ),
      step(
        "Transmit from the peer",
        "Send a known frame from only one endpoint.",
        "cansend can0 123#DEADBEEF",
      ),
    ],
    "The exact frame is received without bus-off, err" +
      "or-passive, or growing error counters.",
    "This test changes interface state. Never connect" +
      " to an active vehicle or production CAN network.",
  ),
  guide(
    "usb",
    "USB",
    "USB host",
    "Connectivity",
    "usb",
    "Interactive",
    "Confirm USB enumeration and controlled read/writ" +
      "e access to removable media.",
    "Known USB device. Storage write tests can destro" +
      "y data and are not included here.",
    [
      step(
        "Inspect enumeration",
        "Capture topology, speed, vendor, and product.",
        "lsusb -t; lsusb",
      ),
      step(
        "Review kernel evidence",
        "Look for enumeration or reset failures.",
        "dmesg | tail -n 80",
      ),
      step(
        "Validate mounted media safely",
        "Read-only inspection first.",
        "lsblk -o NAME,TRAN,SIZE,FSTYPE,MOUNTPOINTS",
      ),
    ],
    "The device enumerates at the expected speed and " +
      "stays connected without repeated resets.",
  ),
  guide(
    "pcie",
    "PCI",
    "PCI Express",
    "Connectivity",
    "pcie",
    "Observe",
    "Confirm the endpoint enumerates and the negotiat" +
      "ed link matches expectations.",
    "Supported endpoint installed before boot.",
    [
      step(
        "List endpoints",
        "Show numeric IDs and kernel drivers.",
        "lspci -nnk",
      ),
      step(
        "Inspect link details",
        "Select the endpoint address from lspci.",
        "lspci -s <bus:device.function> -vv",
      ),
      step(
        "Check kernel messages",
        "Review link training and errors.",
        "dmesg | grep -iE 'pci|pcie|aer'",
      ),
    ],
    "The endpoint is enumerated, bound to the intende" +
      "d driver, and reports no AER or link-training fa" +
      "ilure.",
  ),
  guide(
    "i2c",
    "I²C",
    "I²C buses",
    "Buses & I/O",
    "i2c",
    "Interactive",
    "Confirm adapters and expected peripheral address" +
      "es without writing registers.",
    "Board schematic or expected address map. Some bu" +
      "ses contain sensitive PMIC devices.",
    [
      step(
        "List adapters",
        "Map logical bus numbers to controllers.",
        "i2cdetect -l",
      ),
      step(
        "Scan an approved bus",
        "Replace N only after checking the board document" + "ation.",
        "i2cdetect -y <N>",
      ),
      step(
        "Correlate with drivers",
        "Inspect instantiated kernel clients.",
        "find /sys/bus/i2c/devices -maxdepth 1 -type l -p" + "rintf '%f\\n'",
      ),
    ],
    "Expected addresses appear and kernel-bound devic" +
      "es match the board design.",
    "Scanning a bus is not harmless for every device." +
      " Use only buses approved by the hardware guide.",
  ),
  guide(
    "spi",
    "SPI",
    "SPI bus",
    "Buses & I/O",
    "spi",
    "Interactive",
    "Validate a configured SPI controller and full-du" + "plex transfers.",
    "Approved loopback jumper or known SPI peripheral" +
      "; matching mode, speed, and word size.",
    [
      step(
        "Inspect exposed devices",
        "Confirm the Device Tree enabled the intended con" + "troller.",
        "ls -l /sys/class/spi_master /sys/bus/spi/devices" + " 2>/dev/null",
      ),
      step(
        "Locate spidev",
        "A spidev node exists only when explicitly enable" + "d.",
        "ls -l /dev/spidev*",
      ),
      step(
        "Run the board-approved loopback",
        "Use the exact tool and wiring documented for the" + " carrier.",
        "spidev_test -D /dev/spidev<BUS>.<CS> -v",
      ),
    ],
    "Received bytes match transmitted bytes across th" +
      "e approved speed range.",
  ),
  guide(
    "gpio",
    "GPIO",
    "GPIO lines",
    "Buses & I/O",
    "gpio",
    "Interactive",
    "Map GPIO controllers and validate one approved i" + "nput/output pair.",
    "Schematic, line names, and approved jumper. Neve" +
      "r toggle a power, reset, or boot-strap line.",
    [
      step(
        "Inventory line names",
        "Prefer the GPIO character-device tools.",
        "gpioinfo",
      ),
      step(
        "Observe an input",
        "Replace the chip and line only after checking th" + "e schematic.",
        "gpioget <gpiochipN> <line>",
      ),
      step(
        "Controlled output",
        "Use a temporary value only on an approved test l" + "ine.",
        "gpioset --mode=time --sec=1 <gpiochipN> <line>=1",
      ),
    ],
    "The approved line changes and reads back as desi" +
      "gned, with no unrelated device reset.",
    "Output writes can damage hardware or interrupt t" +
      "he system. VAR-Studio does not execute them autom" +
      "atically.",
  ),
  guide(
    "rtc",
    "RTC",
    "Real-time clock",
    "Platform",
    "rtc",
    "Interactive",
    "Confirm RTC presence, read/write behavior, and r" +
      "etention across a power cycle.",
    "Correct backup supply. Setting time changes syst" + "em and RTC state.",
    [
      step(
        "List RTC devices",
        "Inspect driver and wakeup exposure.",
        "ls -l /sys/class/rtc/; cat /proc/driver/rtc",
      ),
      step(
        "Read hardware time",
        "Read without modifying it.",
        "hwclock --show",
      ),
      step(
        "Retention procedure",
        "Set a known time only in an isolated test, power" +
          " off, wait, and read it after reboot.",
        "hwclock --systohc",
      ),
    ],
    "Time advances correctly and is retained across t" +
      "he specified power-off interval.",
  ),
  guide(
    "camera",
    "CAM",
    "Camera capture",
    "Multimedia",
    "camera",
    "Interactive",
    "Validate V4L2 enumeration, formats, streaming, a" + "nd a captured frame.",
    "Supported sensor or USB camera connected; correc" +
      "t Device Tree and media graph.",
    [
      step(
        "Inspect devices",
        "Identify capture nodes rather than codec or ISP " + "service nodes.",
        "v4l2-ctl --list-devices",
      ),
      step(
        "List formats",
        "Replace the node with the detected capture devic" + "e.",
        "v4l2-ctl -d /dev/video<N> --list-formats-ext",
      ),
      step(
        "Open the visual test",
        "Use Camera or Demo Lab for an on-demand local st" + "ream.",
        "",
      ),
    ],
    "A stable image is captured with the selected res" +
      "olution and no V4L2 timeout appears in dmesg.",
  ),
  guide(
    "audio",
    "AUD",
    "Audio",
    "Multimedia",
    "audio",
    "Interactive",
    "Validate ALSA cards, playback, capture, and an o" +
      "ptional loopback measurement.",
    "Speaker/headphones and microphone or an approved" +
      " loopback cable. Start at low volume.",
    [
      step(
        "Inventory devices",
        "Map ALSA cards and PCM endpoints.",
        "aplay -l; arecord -l",
      ),
      step(
        "Short playback",
        "Choose a known channel and conservative volume.",
        "speaker-test -c 2 -t sine -l 1",
      ),
      step(
        "Capture evidence",
        "Record a short sample to temporary storage.",
        "arecord -d 5 -f S16_LE -r 48000 /tmp/var-studio-a" + "udio.wav",
      ),
    ],
    "Playback and capture use the intended codec, cha" +
      "nnels are mapped correctly, and no underrun/over" +
      "run is reported.",
  ),
  guide(
    "gpu",
    "GPU",
    "GPU graphics",
    "Compute",
    "gpu",
    "Interactive",
    "Validate OpenGL ES rendering while correlating C" +
      "PU, GPU, memory, and thermal behavior.",
    "Display connected and compositor running; suppor" +
      "ted Vivante userspace stack.",
    [
      step(
        "Inspect the GPU collector",
        "Confirm the driver exposes live counters.",
        "gputop",
      ),
      step(
        "Run a controlled workload",
        "Use the Benchmark Lab for allowlisted, time-limi" + "ted demos.",
        "",
      ),
      step(
        "Review evidence",
        "Compare the cropped test window and verify the w" +
          "orkload exited cleanly.",
        "",
      ),
    ],
    "The workload renders correctly, GPU utilization " +
      "responds, and no GPU reset or MMU fault appears.",
  ),
  guide(
    "ml",
    "ML",
    "Machine learning",
    "Compute",
    "npu",
    "Interactive",
    "Validate the installed inference runtime, model " +
      "compatibility, and repeatable latency.",
    "Board-specific runtime and a trusted model bundl" +
      "e. Accelerator availability varies by SoC and im" +
      "age.",
    [
      step(
        "Inspect accelerator exposure",
        "Confirm the expected runtime and device nodes ar" + "e installed.",
        "find /dev /sys/class -maxdepth 3 -iname '*npu*' " +
          "-o -iname '*galcore*' 2>/dev/null",
      ),
      step(
        "Record runtime versions",
        "Use the image package manager or vendor runtime " + "tool.",
        "grep -iE 'npu|tensorflow|onnx|vx' /etc/buildinfo" + " 2>/dev/null",
      ),
      step(
        "Run the packaged example",
        "Use only the model and command supplied for this" + " BSP release.",
        "",
      ),
    ],
    "Inference output is correct and latency is stabl" +
      "e without kernel faults or memory growth.",
  ),
  guide(
    "remoteproc",
    "M4",
    "Remote processor",
    "Compute",
    "remoteproc",
    "Interactive",
    "Inspect Cortex-M/remoteproc availability and val" +
      "idate a packaged firmware example.",
    "Firmware built for the exact SoC core and reserv" +
      "ed-memory layout. Starting firmware changes remo" +
      "te-core state.",
    [
      step(
        "Inspect state",
        "Do not change it before recording the current fi" +
          "rmware and state.",
        "for r in /sys/class/remoteproc/remoteproc*; do e" +
          'cho "$r: $(cat $r/name) $(cat $r/state)"; done',
      ),
      step(
        "Review reserved memory",
        "Confirm Device Tree resources match the firmware" + ".",
        "dmesg | grep -iE 'remoteproc|rpmsg'",
      ),
      step(
        "Use the release procedure",
        "Load/start only a version-matched Variscite exam" + "ple.",
        "",
      ),
    ],
    "Firmware starts, RPMsg endpoints appear when exp" +
      "ected, and shutdown returns the core to a known " +
      "state.",
  ),
  guide(
    "tpm",
    "TPM",
    "TPM security",
    "Security",
    "tpm",
    "Interactive",
    "Confirm TPM discovery and non-destructive capabi" + "lity queries.",
    "TPM enabled in hardware and Device Tree. Avoid c" +
      "lear, ownership, or NV-write operations.",
    [
      step(
        "Inspect kernel exposure",
        "Confirm the class device and driver.",
        "ls -l /sys/class/tpm/; dmesg | grep -i tpm",
      ),
      step(
        "Query capabilities",
        "Read properties without changing ownership.",
        "tpm2_getcap properties-fixed",
      ),
      step(
        "Read random data",
        "Exercise a non-destructive command.",
        "tpm2_getrandom 16 | od -An -tx1",
      ),
    ],
    "The TPM answers capability and random requests w" +
      "ith no transport or self-test error.",
  ),
  guide(
    "mmc",
    "MMC",
    "MMC / SD storage",
    "Storage",
    "mmc",
    "Observe",
    "Confirm controller enumeration, media identity, " +
      "capacity, and error-free I/O.",
    "Known media. Destructive write benchmarks are in" +
      "tentionally excluded.",
    [
      step(
        "Inspect block devices",
        "Map eMMC and SD media without writing.",
        "lsblk -o NAME,PATH,SIZE,ROTA,RO,FSTYPE,MOUNTPOIN" + "TS,MODEL",
      ),
      step(
        "Inspect MMC identity",
        "Review CID/CSD and lifecycle data exposed by sys" + "fs.",
        "find /sys/class/mmc_host -maxdepth 3 -type f \\( " +
          "-name name -o -name cid -o -name csd \\) -print -" +
          "exec cat {} \\;",
      ),
      step(
        "Review errors",
        "Look for CRC, timeout, or tuning failures.",
        "dmesg | grep -iE 'mmc|sdhci'",
      ),
    ],
    "Expected media is present at the correct capacit" +
      "y with no CRC, timeout, or tuning failure.",
  ),
  guide(
    "backlight",
    "BKL",
    "Display and backlight",
    "Multimedia",
    "backlight",
    "Interactive",
    "Validate DRM connectors, display modes, and cont" +
      "rolled backlight brightness.",
    "Connected panel. Brightness changes are visible " +
      "and alter display state.",
    [
      step(
        "Inspect connectors",
        "Confirm connector status and available modes.",
        "for f in /sys/class/drm/card*-*/status; do echo " +
          "$f: $(cat $f); done",
      ),
      step(
        "Inspect backlight range",
        "Record the current and maximum values before cha" + "nging anything.",
        "find /sys/class/backlight -maxdepth 2 -type f -n" +
          "ame '*brightness' -print -exec cat {} \\;",
      ),
      step(
        "Use the release procedure",
        "Change brightness only through the board-approve" +
          "d backlight node.",
        "",
      ),
    ],
    "The expected connector is active, modes match th" +
      "e panel, and brightness changes smoothly without" +
      " display errors.",
  ),
  guide(
    "input",
    "IN",
    "Buttons, touch, and input",
    "Multimedia",
    "input",
    "Interactive",
    "Validate input-device enumeration and observe ke" +
      "y, touch, or switch events.",
    "Known input device and its expected event mappin" + "g.",
    [
      step(
        "List input devices",
        "Map names, handlers, and capabilities.",
        "cat /proc/bus/input/devices",
      ),
      step(
        "Observe events",
        "Select the intended event node, then interact wi" + "th the device.",
        "evtest /dev/input/event<N>",
      ),
      step(
        "Confirm mapping",
        "Compare reported event codes with the carrier-bo" + "ard design.",
        "",
      ),
    ],
    "Every physical action produces the expected even" +
      "t exactly once with no stuck or spurious input.",
  ),
  guide(
    "otg",
    "OTG",
    "USB device / OTG",
    "Connectivity",
    "usb",
    "Interactive",
    "Validate USB role selection and a packaged gadge" + "t function.",
    "Approved USB OTG cable and host PC. Role changes" +
      " disrupt the current USB function.",
    [
      step(
        "Inspect role interfaces",
        "Find controllers that expose host/device role se" + "lection.",
        "find /sys/class/usb_role -maxdepth 2 -type f -pr" +
          "int -exec cat {} \\;",
      ),
      step(
        "Inspect UDC availability",
        "Confirm a gadget-capable controller is exposed.",
        "ls -l /sys/class/udc",
      ),
      step(
        "Run the documented gadget test",
        "Use the exact Variscite procedure for this carri" +
          "er and BSP release.",
        "",
      ),
    ],
    "The board enumerates on the host as the selected" +
      " gadget and transfers data without disconnect lo" +
      "ops.",
  ),
  guide(
    "optee",
    "TEE",
    "OP-TEE",
    "Security",
    "tee",
    "Observe",
    "Confirm the Trusted Execution Environment driver" +
      " and packaged client tests.",
    "OP-TEE enabled in boot firmware, kernel, and roo" + "t filesystem.",
    [
      step(
        "Inspect TEE devices",
        "Confirm kernel class devices and driver messages" + ".",
        "ls -l /sys/class/tee; dmesg | grep -iE 'op-tee|o" + "ptee|tee'",
      ),
      step(
        "Run packaged tests when installed",
        "The package may expose xtest under a release-spe" + "cific path.",
        "command -v xtest && xtest",
      ),
      step(
        "Record versions",
        "Capture boot-firmware and BSP provenance with th" + "e result.",
        "cat /etc/buildinfo",
      ),
    ],
    "The TEE initializes, packaged tests pass, and no" +
      " shared-memory or secure-monitor error appears.",
  ),
  guide(
    "crypto",
    "AES",
    "Crypto acceleration",
    "Security",
    "crypto",
    "Observe",
    "Inventory kernel crypto implementations and comp" +
      "are a controlled algorithm workload.",
    "No external equipment. A benchmark creates tempo" + "rary CPU load.",
    [
      step(
        "Inspect implementations",
        "Distinguish hardware-backed drivers from generic" + " fallbacks.",
        "cat /proc/crypto",
      ),
      step(
        "List AF_ALG support",
        "Confirm the algorithms required by the product.",
        "grep -E '^(name|driver|module)' /proc/crypto",
      ),
      step(
        "Use the release benchmark",
        "Run only the packaged, time-limited crypto test " + "when installed.",
        "var-test test_crypto --help",
      ),
    ],
    "Required algorithms are present, the intended ac" +
      "celerated driver is selected, and the known-answ" +
      "er test passes.",
  ),
  guide(
    "suspend",
    "ZZZ",
    "Suspend and resume",
    "Platform",
    "suspend",
    "Interactive",
    "Validate suspend entry, wake sources, resume, an" +
      "d post-resume peripherals.",
    "Serial console and an approved wake source. Susp" +
      "end interrupts all active sessions.",
    [
      step(
        "Inspect supported states",
        "Do not enter a state until a wake source is conf" + "irmed.",
        "cat /sys/power/state; cat /sys/power/mem_sleep 2" + ">/dev/null",
      ),
      step(
        "Record baseline evidence",
        "Save kernel messages and active interfaces befor" + "e the test.",
        "dmesg | tail -n 100",
      ),
      step(
        "Run from the serial console",
        "Use the release procedure and selected state onl" +
          "y after preparing wake.",
        "echo mem > /sys/power/state",
      ),
    ],
    "The board wakes through the approved source and " +
      "display, network, storage, audio, and timekeepin" +
      "g recover without errors.",
    "This command suspends the board. Never run it th" +
      "rough an unattended remote-only session.",
  ),
  guide(
    "watchdog",
    "WDG",
    "Hardware watchdog",
    "Platform",
    "watchdog",
    "Interactive",
    "Confirm watchdog discovery and a controlled user" +
      "space keepalive path.",
    "Serial console and recovery access. An intention" +
      "al timeout reboots the board.",
    [
      step(
        "Inspect devices",
        "Record identity, status, and timeout range.",
        "ls -l /dev/watchdog* /sys/class/watchdog 2>/dev/" + "null",
      ),
      step(
        "Query safely",
        "Use wdctl when installed; it does not arm the wa" +
          "tchdog by itself.",
        "wdctl /dev/watchdog0",
      ),
      step(
        "Follow the release timeout procedure",
        "Arm only in an isolated test with an explicit re" + "covery plan.",
        "",
      ),
    ],
    "Keepalive prevents reset; the approved timeout t" +
      "est resets once and the subsequent boot contains" +
      " the expected watchdog-reset evidence.",
    "A watchdog test can reboot the board and interru" + "pt storage writes.",
  ),
  guide(
    "iio",
    "ADC",
    "IIO and ADC",
    "Buses & I/O",
    "iio",
    "Observe",
    "Inspect IIO devices and read approved raw sensor" + " or ADC channels.",
    "Known channel scaling and safe signal levels fro" +
      "m the carrier-board guide.",
    [
      step(
        "List devices",
        "Map each IIO device to its kernel name.",
        "for d in /sys/bus/iio/devices/iio:device*; do ec" +
          "ho $d: $(cat $d/name); done",
      ),
      step(
        "List channels",
        "Review raw, scale, and offset attributes.",
        "find /sys/bus/iio/devices/iio:device* -maxdepth " +
          "1 -type f -name 'in_*' -print",
      ),
      step(
        "Read an approved channel",
        "Combine raw, scale, and offset according to the " + "driver ABI.",
        "cat /sys/bus/iio/devices/iio:device<N>/in_<chann" + "el>_raw",
      ),
    ],
    "Readings track the known input, remain within el" +
      "ectrical limits, and use the documented scaling.",
  ),
  guide(
    "docker",
    "CTR",
    "Container runtime",
    "System",
    null,
    "Observe",
    "Confirm Docker availability, architecture, stora" +
      "ge, and an isolated container lifecycle.",
    "Docker installed; image pulls require network ac" +
      "cess and registry policy approval.",
    [
      step(
        "Inspect runtime",
        "Capture server version, architecture, and storag" + "e driver.",
        "docker version; docker info",
      ),
      step(
        "List current state",
        "Do not expose the Docker socket to VAR-Studio.",
        "docker ps -a",
      ),
      step(
        "Use an approved local image",
        "Run a known image already present on the board w" + "hen offline.",
        "docker image ls",
      ),
    ],
    "The daemon is healthy, architecture matches the " +
      "board, and a controlled container starts and exi" +
      "ts without changing host configuration.",
  ),
];
function readiness(item) {
  if (!item.capability)
    return {
      ready: true,
      label: "READY ON THIS IMAGE",
      detail: "No optional capability is required.",
    };
  const cap = state.capabilities.get(item.capability);
  if (cap?.available)
    return {
      ready: true,
      label: "READY ON THIS IMAGE",
      detail: cap.detail,
    };
  return {
    ready: false,
    label: "NEEDS SETUP",
    detail:
      cap?.detail ||
      "The required kernel interface is not exposed by " + "this image.",
  };
}
function visibleGuides() {
  return guides.filter((item) => {
    const r = readiness(item);
    const text = (
      "" +
      item.title +
      " " +
      item.category +
      " " +
      item.objective +
      ""
    ).toLowerCase();
    return (
      (!state.query || text.includes(state.query)) &&
      (state.category === "all" || item.category === state.category) &&
      (state.filter === "all" ||
        (state.filter === "ready" && r.ready) ||
        (state.filter === "setup" && !r.ready))
    );
  });
}
function renderList() {
  const items = visibleGuides();
  $("guide-list").innerHTML = items.length
    ? items
        .map((item) => {
          const r = readiness(item);
          return (
            '<button class="guide-list-item ' +
            (state.selected === item.id ? "active" : "") +
            '" data-id="' +
            item.id +
            '"><span class="guide-icon">' +
            esc(item.icon) +
            '</span><span class="guide-list-copy"><strong>' +
            esc(item.title) +
            "</strong><small>" +
            esc(item.category) +
            " \xB7 " +
            esc(item.mode) +
            '</small></span><i class="readiness-dot ' +
            (r.ready ? "ready" : "setup") +
            '" title="' +
            esc(r.label) +
            '"></i></button>'
          );
        })
        .join("")
    : '<p class="guide-loading">No guides match these f' + "ilters.</p>";
  document
    .querySelectorAll(".guide-list-item")
    .forEach((button) =>
      button.addEventListener("click", () => selectGuide(button.dataset.id)),
    );
}
function renderDetail(item) {
  const r = readiness(item);
  $("guide-detail").innerHTML =
    '<header class="detail-top"><div><p class="eyebro' +
    'w">' +
    esc(item.category.toUpperCase()) +
    " \xB7 " +
    esc(item.mode.toUpperCase()) +
    "</p><h2>" +
    esc(item.title) +
    "</h2><p>" +
    esc(item.objective) +
    '</p></div><span class="readiness-badge ' +
    (r.ready ? "ready" : "setup") +
    '">' +
    esc(r.label) +
    '</span></header><div class="guide-meta"><div><sp' +
    "an>CAPABILITY</span><strong>" +
    esc(r.detail) +
    "</strong></div><div><span>EXECUTION</span><stron" +
    "g>" +
    (item.mode === "Observe"
      ? "Read-only inspection"
      : "User-confirmed in terminal") +
    "</strong></div><div><span>EVIDENCE</span><strong" +
    ">Included in support workflow</strong></div></di" +
    'v><div class="guide-note ' +
    (item.mode === "Observe" ? "safe" : "") +
    '"><strong>Prerequisites:</strong> ' +
    esc(item.prerequisites) +
    "" +
    (item.note ? "<br><strong>Safety:</strong> " + esc(item.note) + "" : "") +
    '</div><div class="test-steps">' +
    item.steps
      .map(
        (s, index) =>
          '<section class="test-step"><span class="step-num' +
          'ber">' +
          (index + 1) +
          '</span><div class="step-body"><h3>' +
          esc(s.title) +
          "</h3><p>" +
          esc(s.text) +
          "</p>" +
          (s.command
            ? '<div class="command-block"><code>' +
              esc(s.command) +
              '</code><button type="button" data-command="' +
              esc(s.command) +
              '">COPY</button></div>'
            : "") +
          "</div></section>",
      )
      .join("") +
    '</div><section class="expected-evidence"><h3>EXP' +
    "ECTED EVIDENCE</h3><p>" +
    esc(item.expected) +
    '</p></section><p class="guide-source">Procedure ' +
    "curated from Variscite board documentation and r" +
    "elease-validation practices. Always verify the c" +
    "arrier-board hardware guide for pin-specific tes" +
    "ts.</p>";
  document
    .querySelectorAll("[data-command]")
    .forEach((button) =>
      button.addEventListener("click", () =>
        copyCommand(button.dataset.command, button),
      ),
    );
}
function selectGuide(id) {
  state.selected = id;
  renderList();
  const item = guides.find((candidate) => candidate.id === id);
  if (item) renderDetail(item);
}
async function copyCommand(command, button) {
  try {
    await navigator.clipboard.writeText(command);
  } catch {
    const area = document.createElement("textarea");
    area.value = command;
    document.body.append(area);
    area.select();
    document.execCommand("copy");
    area.remove();
  }
  const old = button.textContent;
  button.textContent = "COPIED";
  $("copy-toast").classList.add("visible");
  setTimeout(() => {
    button.textContent = old;
    $("copy-toast").classList.remove("visible");
  }, 1400);
}
function setupFilters() {
  const categories = [...new Set(guides.map((item) => item.category))].sort();
  $("category-filter").insertAdjacentHTML(
    "beforeend",
    categories
      .map(
        (category) =>
          '<option value="' +
          esc(category) +
          '">' +
          esc(category) +
          "</option>",
      )
      .join(""),
  );
  $("guide-search").addEventListener("input", (event) => {
    state.query = event.target.value.trim().toLowerCase();
    renderList();
  });
  $("category-filter").addEventListener("change", (event) => {
    state.category = event.target.value;
    renderList();
  });
  document.querySelectorAll("[data-filter]").forEach((button) =>
    button.addEventListener("click", () => {
      document
        .querySelectorAll("[data-filter]")
        .forEach((item) => item.classList.remove("active"));
      button.classList.add("active");
      state.filter = button.dataset.filter;
      renderList();
    }),
  );
}
function updateClock() {
  $("clock").textContent = new Date().toLocaleTimeString([], {
    hour12: false,
  });
}
async function load() {
  try {
    const response = await fetch("/api/v1/diagnostics", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    const data = await response.json();
    state.capabilities = new Map(
      (data.capabilities || []).map((item) => [item.id, item]),
    );
    $("connection").textContent = "Live";
  } catch (error) {
    $("connection").textContent = "Unavailable";
  }
  $("guide-count").textContent = guides.length;
  $("ready-count").textContent = guides.filter(
    (item) => readiness(item).ready,
  ).length;
  renderList();
  selectGuide(guides[0].id);
}
state.guides = guides;
setupFilters();
updateClock();
setInterval(updateClock, 1000);
load();
