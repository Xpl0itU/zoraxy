# v2.6.6 Aug 30 2023

+ Added basic auth editor custom exception rules 
+ Fixed redirection bug under another reverse proxy and Apache location headers [#39](https://github.com/tobychui/zoraxy/issues/39)
+ Optimized memory usage (from 1.2GB to 61MB for low speed geoip lookup) [#52](https://github.com/tobychui/zoraxy/issues/52)
+ Added unset subdomain custom redirection feature [#46](https://github.com/tobychui/zoraxy/issues/46)
+ Fixed potential security issue in satori/go.uuid [#55](https://github.com/tobychui/zoraxy/issues/55)
+ Added custom acme feature in back-end, thx [@daluntw](https://github.com/daluntw)
+ Added bypass TLS check for custom acme server, thx [@daluntw](https://github.com/daluntw)
+ Introduce new startparameter `-fastgeoip=true`, see [Releases](https://github.com/tobychui/zoraxy/releases/tag/2.6.6)

# v2.6.5.1 Jul 26 2023

+ Patch on memory leaking for Windows netstat module (do not effect any of the previous non Windows builds)
+ Fixed potential memory leak in acme handler logic
+ Added "Do you want to get a TLS certificate for this subdomain?" dialog when a new subdomain proxy rule is created

# v2.6.5 Jul 19 2023

+ Added Import / Export-Feature 
+ Moved configurationfiles to a separate folder [#26](https://github.com/tobychui/zoraxy/issues/26)
+ Added auto-renew with ACME [#6](https://github.com/tobychui/zoraxy/issues/6)
+ Fixed Whitelistbug [#18](https://github.com/tobychui/zoraxy/issues/18)
+ Added Whois

# v2.6.4 Jun 15 2023

+ Added force TLS v1.2 above toggle
+ Added trace route
+ Added ICMP ping
+ Added special routing rules module for up-coming acme integration
+ Fixed IPv6 check bug in black/whitelist
+ Optimized UI for TCP Proxy

# v2.6.3 Jun 8 2023

+ Added X-Forwarded-Proto for automatic proxy detector
+ Split blacklist and whitelist from geodb script file
+ Optimized compile binary size
+ Added access control to TCP proxy
+ Added "invalid config detect" in up time monitor for isse [#7](https://github.com/tobychui/zoraxy/issues/7)
+ Fixed minor bugs in advance stats panel
+ Reduced file size of embedded materials

# v2.6.2 Jun 4 2023

+ Added advance stats operation tab
+ Added statistic reset [#13](https://github.com/tobychui/zoraxy/issues/13)
+ Added statistic export to csv and json (please use json)
+ Make subdomain clickable (not vdir) [#12](https://github.com/tobychui/zoraxy/issues/12)
+ Added TCP Proxy
+ Updates SMTP setup UI to make it more straight forward to setup

# v2.6.1 May 31 2023

+ Added reverse proxy TLS skip verification
+ Added basic auth
+ Edit proxy settings
+ Whitelist
+ TCP Proxy (experimental)
+ Info (Utilities page)

# v2.6 May 27 2023

+ Basic auth
+ Support TLS verification skip (for self signed certs)
+ Added trend analysis
+ Added referer and file type analysis
+ Added cert expire day display
+ Moved subdomain proxy logic to dpcore
