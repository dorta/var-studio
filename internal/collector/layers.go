package collector

func layerLinks(name, revision string) (string, string) {
	githubRepos := map[string]string{
		"meta": "yoctoproject/poky", "meta-poky": "yoctoproject/poky",
		"meta-oe": "openembedded/meta-openembedded",
			"meta-multimedia": "openembedded/meta-openembedded",
		"meta-python": "openembedded/meta-openembedded",
			"meta-gnome": "openembedded/meta-openembedded",
		"meta-networking": "openembedded/meta-openembedded",
			"meta-filesystems": "openembedded/meta-openembedded",
		"meta-freescale": "Freescale/meta-freescale",
			"meta-freescale-3rdparty": "Freescale/meta-freescale-3rdparty",
		"meta-freescale-distro": "Freescale/meta-freescale-distro",
			"meta-chromium": "OSSystems/meta-browser",
		"meta-swupdate": "sbabic/meta-swupdate", "meta-qt6": "varigit/meta-qt6",
			"meta-clang": "kraj/meta-clang",
		"meta-variscite-bsp-imx": "varigit/meta-variscite-bsp-imx",
			"meta-variscite-sdk-imx": "varigit/meta-variscite-sdk-imx",
		"meta-variscite-bsp-common": "varigit/meta-variscite-bsp-common",
			"meta-variscite-sdk-common": "varigit/meta-variscite-sdk-common",
		"meta-variscite-hab": "varigit/meta-variscite-hab",
		"meta-imx-bsp":       "nxp-imx/meta-imx", "meta-imx-sdk": "nxp-imx/meta-imx",
		"meta-imx-ml": "nxp-imx/meta-imx", "meta-imx-v2x": "nxp-imx/meta-imx",
		"meta-nxp-matter-baseline": "nxp-imx/meta-nxp-connectivity",
			"meta-nxp-openthread": "nxp-imx/meta-nxp-connectivity",
		"meta-nxp-demo-experience": "nxp-imx-support/meta-nxp-demo-experience",
	}
	if repo, ok := githubRepos[name]; ok {
		base := "https://github.com/" + repo
		return base, base + "/commit/" + revision
	}

	yoctoRepos := map[string]string{
		"meta-arm": "meta-arm", "meta-arm-toolchain": "meta-arm",
		"meta-parsec": "meta-security", "meta-tpm": "meta-security",
		"meta-virtualization": "meta-virtualization",
	}
	if repo, ok := yoctoRepos[name]; ok {
		base := "https://git.yoctoproject.org/" + repo
		return base, base + "/commit/?id=" + revision
	}
	return "", ""
}
