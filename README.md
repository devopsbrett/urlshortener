# URLShortener

A simple URLShortener service.

## Running & Testing

The initial implementation of this service ships with support for 2 different datastores, a builtin key value store that persists to the filesystem (badgerdb), and support for an external key value store (redis). BadgerDB allows for easy testing on your local system while allowing production deployments using redis allows for load balancing the service while sharing a datastore between each of the instances. A docker compose file exists for each of these setups (**Requires docker compose version >= 2.0.0 with support for compose file version 3.9**)

To launch using the badgerdb store run: `docker compose up -d`

To launch using the redis store run: `docker compose -f docker-compose.redis.yml up -d`

Once running you can test the setup using

`docker compose -f [docker-compose.yml|docker-compose.redis.yml] -f docker-compose.testing.yml up`

By default this will test using 100 requests per second for 5 minutes. The rate requests are made can be modified by setting the environment variable `REQ_RATE` eg.

`REQ_RATE=50 docker compose -f docker-compose.redis.yml -f docker-compose.testing.yml up`

When running the tests on a linux server with 2GB RAM and 4 vcpus running the tests for 5 minutes with 1000 requests per second generated the following results.

```
vegeta_1        | Requests      [total, rate, throughput]         300000, 1000.00, 1000.00
vegeta_1        | Duration      [total, attack, wait]             5m0s, 5m0s, 1.648ms
vegeta_1        | Latencies     [min, mean, 50, 90, 95, 99, max]  613.915µs, 2.383ms, 1.832ms, 3.088ms, 4.085ms, 8.75ms, 322.772ms
vegeta_1        | Bytes In      [total, mean]                     10200000, 34.00
vegeta_1        | Bytes Out     [total, mean]                     0, 0.00
vegeta_1        | Success       [ratio]                           100.00%
vegeta_1        | Status Codes  [code:count]                      200:300000
vegeta_1        | Error Set:
```
## Exposed Paths

To shorten a URL you have 2 options:

`POST /api/v1/shorten/*url` eg. `http://localhost:5000/api/v1/shorten/gmail.com`

or

`POST /api/v1/shorten` and setting the url parameter. eg

`curl -XPOST http://localhost:5000/api/v1/shorten -d 'url=gmail.com/#blah'`

The latter was added in order to support some SPA (Like VueJS using the default VueRouter) as the **#** fragment is deamed to be client side and not passed on to the webserver.

To get info about a short URL you can issue the request

`GET /api/v1/lookup/:id`


Finally `GET /:id` will give a `HTTP/1.1 308 Permanent Redirect` and will redirect you to the correct site.

## Implementation / Design Decisions

IDs are made up of at least nine characters using a base62 character set [0-9a-zA-Z] that has been pre shuffled for some added entropy.

### How are the short URL IDs Generated

Currently the IDs are made up of 8 characters (the prefix) which is generated using a deterministic function using the full URL as the input, followed by an iterator character(s) to allow for collisions as well as limiting the records we need to search to see if the URL has been stored previously.

The deterministic function used to generate the prefix is PBKDF2 (With a very low iteration count as it's not being used to generate a cryptographically secure key, we just want a deterministic way of jumbling the input data while keeping response times low) and SHA1 as the hash function. The other benefit of PBKFD2 is that it is used as a key stretching algorithm, allowing you to set the amount of bytes it returns. In this case we can consider it more of a key shrinking algorithm, as we only need 6 bytes. The reason for this is that 6 bytes (base256) covers every possible combination that 8 characters (base62) could cover (and a bit more).

<!-- $$
\begin{align*}
256^6 &= 281,474,976,710,656 \\
62^8 &= 218,340,105,584,896 \\
62^8 - 1 &= \texttt{C6 94 44 6F 01 00}
\end{align*}
$$ --> 

<div align="center"><img src="svg/v6crVghpXe.svg"></div>

As shown in the above calculations we would need to make sure the function never returned more a result greater than 0xc694446f0100. This could be done using modulus (which would also mean values 00000000 - hVwxnaA7 [Base62] would be generated twice as often as any value above that), however rather than perform math operation against a uint64 value this implemetation sacrifices some potential prefixes and concentrates on the first byte (B[0]) from the result of the PBKFD2 function and applying the following, while leaving the other bytes as they are:

<!-- $$
\frac{\texttt{B[0]}}{256} * \texttt{C6}
$$ --> 

<div align="center"><img src="svg/2e0yd3My5g.svg"></div>

This does mean that there are 58 values that will be twice as likely to be selected as the first byte but these are distributed through all potential values. Assuming that each of the 6 bytes returned from PBKFD2 have an equal chance of being 0x00 - 0xff, and taking into account the 2 character iterator we can calculate the possible number of URL we can shorten:

<!-- $$
\begin{align*}
62^2 &= 3844\\
\texttt{0xc6} - 58 &= 140\\
140 * 256^5 * 3844 &= 591713177603932160\\
58 * 256^5 * \frac{3844}{2} &= 122569158217957376\\
&= 714282335821889536
\end{align*}
$$ --> 

<div align="center"><img src="svg/b5ol6xElnD.svg"></div>


Which would mean should this service be extreamly well used and see 1000 new URLs each second we shouldn't run out of available IDs for 22649744 years (And hopefully within that time someone would be able to figure out a way of using all those IDs I had to discard [or maybe we'll have dinosaurs again. I'm hoping for the dinos])