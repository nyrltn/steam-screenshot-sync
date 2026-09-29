Name:           steam-screenshot-sync
Version:        0.1.0
Release:        1%{?dist}
Summary:        Hard-link Steam screenshots into ~/Pictures/Steam per game
License:        MIT
Source0:        %{name}-%{version}.tar.gz

# Plain Go module with no dependencies: build in module mode, no vendoring.
%global gomodulesmode GO111MODULE=on

ExclusiveArch:  %{golang_arches}
BuildRequires:  golang >= 1.26
BuildRequires:  go-rpm-macros
BuildRequires:  systemd-rpm-macros
%{?systemd_requires}

%description
A small utility run by a systemd user timer. It scans the Steam
screenshot folders and hard-links every screenshot into
~/Pictures/Steam/<Game Name>/, so the pictures survive when Steam's
own copies are deleted. Across filesystems it copies instead.

%prep
%autosetup

%build
export GOFLAGS="-mod=readonly"
export LDFLAGS="-X main.version=%{version}"
%gobuild -o %{name} .

%check
go test ./...

%install
install -Dpm0755 %{name} %{buildroot}%{_bindir}/%{name}
install -Dpm0644 systemd/%{name}.service %{buildroot}%{_userunitdir}/%{name}.service
install -Dpm0644 systemd/%{name}.timer %{buildroot}%{_userunitdir}/%{name}.timer
install -Dpm0644 systemd/%{name}.preset %{buildroot}%{_userpresetdir}/90-%{name}.preset

%post
%systemd_user_post %{name}.timer

%preun
%systemd_user_preun %{name}.timer

%postun
%systemd_user_postun_with_restart %{name}.timer

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_userunitdir}/%{name}.service
%{_userunitdir}/%{name}.timer
%{_userpresetdir}/90-%{name}.preset

%changelog
* Tue Sep 29 2026 nyrltn <johalic12@gmail.com> - 0.1.0-1
- Initial package
