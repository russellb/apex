Name: apex
Summary: Apex
Group: System Environment/Daemons
URL: https://github.com/redhat-et/apex
Version: %{_version}
Release: %{_release}%{?dist}
License: ASL 2.0
Source: apex-%{version}.%{_release}.tar.gz

BuildRequires: golang
Requires: wireguard-tools
Requires(post): systemd-units
Requires(preun): systemd-units
Requires(postun): systemd-units

%define _build_id_links none

%description
This is the local daemon for connecting a host to the Apex connectivity service.

%prep
%autosetup

%install
install -p -D -m 0755 dist/apex-linux-amd64 $RPM_BUILD_ROOT%{_bindir}/apex
install -p -D -m 0755 dist/apexd-linux-amd64 $RPM_BUILD_ROOT%{_bindir}/apexd
install -p -D -m 0644 contrib/rpm/apex.service $RPM_BUILD_ROOT%{_unitdir}/apex.service
install -p -D -m 0644 contrib/rpm/apex.sysconfig $RPM_BUILD_ROOT%{_sysconfdir}/sysconfig/apex

%files
%{_bindir}/apex
%{_bindir}/apexd
%{_unitdir}/apex.service
%{_sysconfdir}/sysconfig/apex

%post
%systemd_post apex.service

%preun
%systemd_preun apex.service

%postun
%systemd_postun apex.service

%changelog
* Mon Jan 23 2023 Russell Bryant <rbryant@redhat.com>
- Initial spec file for apex agent